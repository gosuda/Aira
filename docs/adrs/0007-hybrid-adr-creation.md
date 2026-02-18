# ADR-0007: Hybrid ADR Creation (Agent Tool + Post-Processing)

## Status

Accepted

## Date

2026-02-17

## Context

Aira's ADR linear history (ADR-0003) depends on AI agents producing structured decision records for every architectural choice they make during task execution. Two fundamental challenges exist:

1. **Explicit decisions:** Agents sometimes make deliberate architectural choices (e.g., "I'll use Redis for caching") that they can be trained to record via a tool call. However, agents may forget to call the tool, or the tool call may fail silently.

2. **Implicit decisions:** Agents frequently make decisions without recognizing them as architectural choices (e.g., choosing a particular error handling pattern, selecting a data structure, or establishing a naming convention). These decisions are visible in the code diff and conversation history but were never explicitly recorded.

Relying on a single mechanism (tool-only or extraction-only) leaves gaps in ADR coverage.

## Decision

Implement a **dual-mechanism** approach for ADR creation:

### Mechanism 1: Explicit agent tool (`create_adr`)

- Agents have access to a `create_adr` MCP tool that they can call during execution.
- The tool accepts structured input: `title`, `context`, `decision`, `alternatives` (optional), `consequences` (optional).
- The tool immediately writes the ADR to the database and returns the ADR sequence number to the agent.
- Agent system prompts instruct agents to call `create_adr` whenever they make an architectural choice.
- This mechanism produces the highest-quality ADRs because the agent records the decision at the moment it is made, with full context.

### Mechanism 2: Post-processing extraction

- After task completion (agent session ends), a post-processing pipeline runs.
- The pipeline sends the agent's conversation history and the git diff to an LLM with a structured extraction prompt.
- The LLM identifies architectural decisions that were made but not explicitly recorded via the `create_adr` tool.
- Extracted decisions are formatted as ADR records and inserted into the database.
- A dedup step compares extracted ADRs against existing ADRs for the same task session (by title similarity and decision content overlap) to prevent duplicates.

Both mechanisms feed into the same ADR linear history (ADR-0003).

## Alternatives Considered

### Alternative 1: Agent tool only

- Rely entirely on agents calling the `create_adr` tool for every architectural decision.
- **Rejected because:** Agents miss implicit decisions they do not recognize as architectural. Even explicit decisions may be skipped when the agent is focused on implementation. Tool call failures (network, timeout) would silently drop decisions. No safety net for incomplete recording.

### Alternative 2: Post-processing only

- Analyze the complete conversation and diff after task completion to extract all decisions.
- **Rejected because:** Post-processing produces less structured and less accurate ADRs than real-time tool calls. The LLM performing extraction has less context than the agent that made the decision. All ADR creation is delayed until after task completion, preventing real-time visibility on the kanban board during execution.

### Alternative 3: Manual ADR creation by humans

- Require human team members to write ADRs based on reviewing agent work.
- **Rejected because:** Defeats the automation purpose of Aira. Humans reviewing agent code diffs to write ADRs is more work than writing the code themselves. Scales poorly with agent task volume.

## Consequences

### Positive

- Higher ADR coverage: explicit tool calls capture deliberate decisions, post-processing catches implicit ones.
- Structured data from explicit `create_adr` calls produces high-quality ADRs with full context.
- Post-processing acts as a safety net, ensuring no significant decision goes unrecorded.
- Real-time ADR creation (via tool) enables live kanban board updates during agent execution.

### Negative

- Post-processing adds latency after task completion: the extraction pipeline must process the conversation history and diff before ADRs appear.
- Extraction requires an LLM call (additional API cost per task completion), which scales with task volume.
- Dedup logic between tool-created and extraction-created ADRs is heuristic (title similarity, content overlap) and may produce false positives (merging distinct but similar decisions) or false negatives (allowing near-duplicate ADRs).
- Two creation paths increase testing surface: both must be verified for correct ADR format, database insertion, and kanban board propagation.

### Neutral

- The relative proportion of tool-created vs. extracted ADRs will depend on agent system prompt quality and can be monitored to improve prompts over time.
- Post-processing extraction quality will improve as the extraction prompt is refined based on real-world agent conversations.

## References

- ADR-0003: ADR Linear History with Task Breakdown
- [MCP (Model Context Protocol) Tool Specification](https://modelcontextprotocol.io/)
