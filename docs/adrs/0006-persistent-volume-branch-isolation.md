# ADR-0006: Persistent Volume with Git Worktree Isolation for Repo Access

## Status

Accepted

## Date

2026-02-17

## Context

AI agents operating on code repositories need read-write access to the repository files. In a multi-tenant environment, multiple agents may work on the same repository concurrently (e.g., one agent implementing a feature while another fixes a bug). The repository access mechanism must satisfy:

- **Performance:** Cloning large repositories (1GB+) for every task is unacceptably slow (10-30 seconds).
- **Concurrency safety:** Multiple agents working on the same repository must not corrupt each other's working trees.
- **Isolation:** Each agent session must have its own independent working directory with its own branch and file state.
- **Network access:** Agents need full internet access to install dependencies, pull Docker images, and access APIs.

This ADR was revised during implementation when the original plan (`git checkout -b` on a shared working tree) was found to be unsafe for concurrent access.

## Decision

Use **persistent Docker volumes** for repository storage combined with **git worktree** for per-session isolation:

1. **Persistent volume per project:** Each project's repository is cloned once into a dedicated Docker volume. On subsequent task starts, `git fetch` updates the local repository (typically < 2 seconds vs. 10-30 seconds for a full clone).

2. **Git worktree per agent session:** Each agent session gets an isolated working directory via `git worktree add -b aira/<session-id> <worktree-path>`. The worktree path follows the convention `/repo/.worktrees/<branch-name>`. This gives each agent:
   - Its own branch (`aira/<session-id>`) for commits.
   - Its own working tree (independent file state).
   - Shared git objects database (saves disk space, no redundant clone).

3. **On completion:** The agent opens a pull request or merges to the target branch, depending on the task configuration.

4. **Cleanup:** `git worktree remove` is called on session completion or cancellation. `RemoveWorktree` is registered as a lifecycle hook on the agent session.

5. **Network access:** Agent containers have full internet access (no network restrictions) to support real-world development tasks (package installation, API calls, etc.).

## Alternatives Considered

### Alternative 1: Clone-per-task

- Perform a full `git clone` for each agent session, providing natural isolation via separate repository copies.
- **Rejected because:** Clone time for large repositories (10-30 seconds) adds unacceptable latency to task startup. Disk usage scales linearly with concurrent sessions, as each clone duplicates the entire git objects database. Network bandwidth is wasted re-downloading the same objects.

### Alternative 2: Git checkout -b on shared working tree (original plan)

- Use a single shared working tree per project, with each agent running `git checkout -b <branch>` before starting work.
- **Rejected because:** `git checkout` modifies the working tree in-place. When multiple agents share the same working tree, one agent's checkout overwrites the files another agent is actively editing. This was discovered during implementation when concurrent agent sessions produced corrupted file states. Git worktrees solve this by providing physically separate working directories backed by the same repository.

### Alternative 3: No-network containers

- Run agent containers with `--network none` to prevent data exfiltration and dependency on external services.
- **Rejected because:** Too restrictive for real development tasks. Agents need to run `npm install`, `go mod download`, `pip install`, pull container images, access documentation, and interact with APIs. Pre-loading all possible dependencies into the container image is impractical given the variety of project stacks.

## Consequences

### Positive

- Fast agent startup: `git fetch` + `git worktree add` completes in approximately 2 seconds, compared to 10-30 seconds for a full clone.
- Multiple agents work on the same repository simultaneously without interfering with each other's file state.
- Shared git objects database saves significant disk space compared to full clones per session.
- Clean isolation model: each agent has its own branch and working directory, making it easy to reason about state.
- Full internet access enables agents to perform realistic development tasks.

### Negative

- Docker volume management adds operational complexity: volumes must be created, mounted, and cleaned up correctly.
- Stale volumes (from deleted projects or failed cleanup) must be detected and garbage-collected.
- Worktree cleanup must be reliable: a crashed agent session that fails to clean up its worktree leaves behind stale branches and worktree references that accumulate over time.
- Branch naming conflicts are theoretically possible if session IDs collide (mitigated by UUIDs).

### Neutral

- The persistent volume approach means the first task on a project incurs a full clone cost, but all subsequent tasks benefit from fast fetch-only updates.
- Git worktree is a standard Git feature (introduced in Git 2.5) and does not require custom tooling.

## References

- [git-worktree documentation](https://git-scm.com/docs/git-worktree)
- ADR-0002: Pluggable Agent Interface with Docker Execution (container lifecycle)
- Original implementation note: migrated from `git checkout -b` to `git worktree add` after concurrency safety issue was identified
