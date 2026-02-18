package slack

import (
	"regexp"
	"strings"
)

// CommandAction represents the type of parsed command.
type CommandAction string

const (
	// CommandActionTask indicates a task creation intent.
	CommandActionTask CommandAction = "task"
	// CommandActionStatus indicates a status query.
	CommandActionStatus CommandAction = "status"
	// CommandActionHelp indicates a help request.
	CommandActionHelp CommandAction = "help"
	// CommandActionLink indicates an account linking request.
	CommandActionLink CommandAction = "link"
	// CommandActionUnknown indicates an unrecognized or empty command.
	CommandActionUnknown CommandAction = "unknown"
)

// Command represents a parsed user command from Slack.
type Command struct {
	Action CommandAction
	Title  string // for task commands
	Raw    string // original text
}

// mentionPattern matches both Slack-encoded mentions (<@U12345>) and literal @aira at the start.
var mentionPattern = regexp.MustCompile(`^(?:<@[A-Z0-9]+>|@aira)\s*`) //nolint:gochecknoglobals // compiled regexp

// createTaskPattern matches "create task:" prefix (case-insensitive).
var createTaskPattern = regexp.MustCompile(`(?i)^create\s+task:\s*`) //nolint:gochecknoglobals // compiled regexp

// ParseCommand extracts a command from a Slack message text.
// It handles both "@aira ..." and "<@U12345> ..." formats (Slack encodes mentions).
func ParseCommand(text string) Command {
	cmd := Command{
		Action: CommandActionUnknown,
		Raw:    text,
	}

	// Strip the mention prefix.
	stripped := mentionPattern.ReplaceAllString(text, "")
	stripped = strings.TrimSpace(stripped)

	// If nothing was stripped (no mention found) or text is empty after stripping,
	// and the original text had no mention, return unknown.
	if stripped == strings.TrimSpace(text) || stripped == "" {
		// Check if the original had a mention; if not, it's unknown.
		if !mentionPattern.MatchString(text) {
			return cmd
		}
		// Had a mention but nothing after it: unknown.
		if stripped == "" {
			return cmd
		}
	}

	// Check single-word keywords.
	lower := strings.ToLower(stripped)
	switch lower {
	case "status":
		cmd.Action = CommandActionStatus
		return cmd
	case "help":
		cmd.Action = CommandActionHelp
		return cmd
	case "link":
		cmd.Action = CommandActionLink
		return cmd
	}

	// Check for "create task:" prefix.
	if loc := createTaskPattern.FindStringIndex(stripped); loc != nil {
		title := strings.TrimSpace(stripped[loc[1]:])
		if title != "" {
			cmd.Action = CommandActionTask
			cmd.Title = title
			return cmd
		}
	}

	// Default: treat remaining text as a task title.
	cmd.Action = CommandActionTask
	cmd.Title = stripped

	return cmd
}
