package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
)

// decisionPattern captures common decision phrasing in agent conversation logs.
// Each pattern expects at least one named group: "chosen" and optionally "rejected".
var decisionPatterns = []*regexp.Regexp{ //nolint:gochecknoglobals // compiled regexps
	// "chose X over Y"
	regexp.MustCompile(`(?i)chose\s+(?P<chosen>[^.]+?)\s+over\s+(?P<rejected>[^.]+)`),
	// "decided to use X" / "decided to go with X"
	regexp.MustCompile(`(?i)decided\s+to\s+(?:use|go\s+with|adopt|implement)\s+(?P<chosen>[^.]+)`),
	// "switched from X to Y"
	regexp.MustCompile(`(?i)switched\s+from\s+(?P<rejected>[^.]+?)\s+to\s+(?P<chosen>[^.]+)`),
	// "selected X instead of Y"
	regexp.MustCompile(`(?i)selected\s+(?P<chosen>[^.]+?)\s+instead\s+of\s+(?P<rejected>[^.]+)`),
	// "replaced X with Y"
	regexp.MustCompile(`(?i)replaced\s+(?P<rejected>[^.]+?)\s+with\s+(?P<chosen>[^.]+)`),
}

// detectedDecision holds a decision extracted from conversation text.
type detectedDecision struct {
	Chosen   string
	Rejected string
	Source   string // the original line containing the decision
}

// PostProcessor analyzes agent conversation and git diff after task completion
// to extract implicit architectural decisions.
type PostProcessor struct {
	adrs     domain.ADRRepository
	sessions domain.AgentSessionRepository
}

// NewPostProcessor creates a PostProcessor with the required repositories.
func NewPostProcessor(adrs domain.ADRRepository, sessions domain.AgentSessionRepository) *PostProcessor {
	return &PostProcessor{
		adrs:     adrs,
		sessions: sessions,
	}
}

// ExtractImplicitADRs scans conversation logs and git diff for decision patterns,
// deduplicates against existing ADRs, and creates draft ADRs for new decisions.
// Returns the number of ADRs created and any error encountered.
func (p *PostProcessor) ExtractImplicitADRs(
	ctx context.Context,
	sessionID uuid.UUID,
	tenantID uuid.UUID,
	projectID uuid.UUID,
	conversationLog []string,
	gitDiff string,
) (int, error) {
	// Combine conversation log and diff into searchable lines.
	lines := make([]string, 0, len(conversationLog)+1)
	lines = append(lines, conversationLog...)
	if gitDiff != "" {
		// Only scan diff comment lines (+ lines that might contain decisions in commit messages).
		for diffLine := range strings.SplitSeq(gitDiff, "\n") {
			trimmed := strings.TrimSpace(diffLine)
			if strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(trimmed, "+++") {
				lines = append(lines, strings.TrimPrefix(trimmed, "+"))
			}
		}
	}

	// Extract decisions via pattern matching.
	decisions := extractDecisions(lines)
	if len(decisions) == 0 {
		return 0, nil
	}

	// Load existing ADRs for deduplication.
	existingADRs, err := p.adrs.ListByProject(ctx, tenantID, projectID)
	if err != nil {
		return 0, fmt.Errorf("agent.PostProcessor.ExtractImplicitADRs: list existing: %w", err)
	}

	existingTitles := make(map[string]struct{}, len(existingADRs))
	for _, adr := range existingADRs {
		existingTitles[normalizeTitle(adr.Title)] = struct{}{}
	}

	created := 0
	for _, d := range decisions {
		title := buildDecisionTitle(d)
		if _, exists := existingTitles[normalizeTitle(title)]; exists {
			continue
		}

		// Allocate sequence and create draft ADR.
		seq, seqErr := p.adrs.NextSequence(ctx, projectID)
		if seqErr != nil {
			return created, fmt.Errorf("agent.PostProcessor.ExtractImplicitADRs: next sequence: %w", seqErr)
		}

		now := time.Now()
		adr := &domain.ADR{
			ID:        uuid.New(),
			TenantID:  tenantID,
			ProjectID: projectID,
			Sequence:  seq,
			Title:     title,
			Status:    domain.ADRStatusDraft,
			Context:   "Automatically detected from agent conversation: " + d.Source,
			Decision:  "Use " + d.Chosen,
			Drivers:   nil,
			Options:   buildOptions(d),
			Consequences: domain.ADRConsequences{
				Good:    nil,
				Bad:     nil,
				Neutral: []string{"Extracted automatically; requires human review."},
			},
			CreatedBy:      nil,
			AgentSessionID: &sessionID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if createErr := p.adrs.Create(ctx, adr); createErr != nil {
			return created, fmt.Errorf("agent.PostProcessor.ExtractImplicitADRs: create ADR: %w", createErr)
		}

		// Track title to avoid duplicates within this batch.
		existingTitles[normalizeTitle(title)] = struct{}{}
		created++
	}

	return created, nil
}

// extractDecisions runs all decision patterns against the provided lines
// and returns deduplicated decisions.
func extractDecisions(lines []string) []detectedDecision {
	seen := make(map[string]struct{})
	var results []detectedDecision

	for _, line := range lines {
		for _, pat := range decisionPatterns {
			match := pat.FindStringSubmatch(line)
			if match == nil {
				continue
			}

			d := detectedDecision{Source: strings.TrimSpace(line)}

			for i, name := range pat.SubexpNames() {
				switch name {
				case "chosen":
					d.Chosen = strings.TrimSpace(match[i])
				case "rejected":
					d.Rejected = strings.TrimSpace(match[i])
				}
			}

			if d.Chosen == "" {
				continue
			}

			// Deduplicate by normalized chosen value.
			key := normalizeTitle(d.Chosen)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			results = append(results, d)
		}
	}

	return results
}

// buildDecisionTitle creates a human-readable ADR title from a detected decision.
func buildDecisionTitle(d detectedDecision) string {
	if d.Rejected != "" {
		return "Use " + d.Chosen + " instead of " + d.Rejected
	}
	return "Use " + d.Chosen
}

// buildOptions creates the options list from a detected decision.
func buildOptions(d detectedDecision) []string {
	opts := []string{d.Chosen}
	if d.Rejected != "" {
		opts = append(opts, d.Rejected)
	}
	return opts
}

// normalizeTitle lowercases and trims a title for deduplication comparison.
func normalizeTitle(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
