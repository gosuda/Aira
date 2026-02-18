# ADR-0002: Pluggable Agent Interface with Docker Execution

## Status

Accepted

## Date

2026-02-17

## Context

Aira must support multiple AI agent frameworks (Claude Code SDK, Codex CLI, OpenCode, ACP-compatible agents) to execute development tasks on behalf of users. Each framework has distinct invocation patterns, output formats, and lifecycle requirements. In a multi-tenant SaaS environment, agent execution must be strongly isolated between tenants to prevent data leakage, resource starvation, and security breaches.

Key forces:

- **Multi-framework support** requires a common abstraction that accommodates different agent protocols without leaking framework-specific details into business logic.
- **Tenant isolation** demands that one tenant's agent execution cannot access another tenant's files, environment variables, secrets, or network resources.
- **Resource control** (CPU, memory, disk) must be enforceable per agent session to prevent runaway processes from affecting other tenants.
- **Lifecycle management** must handle agent startup, health monitoring, output streaming, graceful shutdown, and crash recovery uniformly across all frameworks.

## Decision

Define an `AgentBackend` Go interface that all agent framework integrations must implement. Each agent session runs in an **isolated Docker container** with a persistent repository volume mount. Supporting abstractions:

- **`AgentBackend` interface** defines `Start(ctx, config) (Session, error)`, `Stop(ctx, sessionID) error`, `StreamOutput(ctx, sessionID) (<-chan OutputEvent, error)`, and `Health(ctx) error`. Each agent framework (Claude, Codex, OpenCode) implements this interface.
- **`AgentRegistry`** is a factory that creates the appropriate `AgentBackend` based on agent type string. Registration happens at startup via `Register(agentType string, factory AgentBackendFactory)`.
- **`TransportHandler` interface** handles per-agent protocol quirks: JSON parsing variations, output filtering, timeout policies, and error code mapping. Each `AgentBackend` implementation composes a `TransportHandler`.
- **Docker container per session** provides filesystem isolation, network namespace isolation, and cgroup-based resource limits (CPU shares, memory limit, PIDs limit). The container mounts the project's persistent volume at a fixed path.

## Alternatives Considered

### Alternative 1: Subprocess execution (os/exec)

- Run agent CLI tools directly as subprocesses on the host machine.
- **Rejected because:** No isolation between tenants sharing the same host. A malicious or buggy agent process can access the host filesystem, environment variables, and network. Resource limits via cgroups require root-level setup outside Go. Process cleanup on crash is unreliable without a container runtime managing the lifecycle.

### Alternative 2: SDK in-process execution

- Import agent SDKs as Go libraries and run them in-process within goroutines.
- **Rejected because:** Multi-tenant execution in a single process is a security risk: shared memory space means one tenant's agent could theoretically access another's data. Agent framework bugs (memory leaks, panics) would crash the entire Aira process. Most agent frameworks are CLI tools or Python-based, not importable Go libraries.

### Alternative 3: Remote workers via message queue

- Deploy agent workers as separate services consuming tasks from a message queue (NATS, RabbitMQ).
- **Rejected because:** Introduces significant infrastructure complexity (queue deployment, worker scaling, message serialization, dead letter handling) that is unjustified for MVP. The indirection makes debugging harder and adds latency to agent session lifecycle operations. Can be adopted later if horizontal scaling demands it.

## Consequences

### Positive

- Strong tenant isolation via Docker containers: filesystem, network, PID, and resource isolation out of the box.
- Adding a new agent framework requires implementing one Go interface (`AgentBackend`) and registering it with the factory.
- Resource limits (CPU, memory, PIDs) are enforced per container, preventing resource starvation across tenants.
- Clean lifecycle management: Docker handles process supervision, crash detection, and cleanup.
- `TransportHandler` absorbs framework-specific protocol differences, keeping business logic framework-agnostic.

### Negative

- Docker daemon dependency required on all hosts running Aira, including self-hosted deployments.
- Container startup latency of approximately 2-5 seconds per agent session due to image pull (first run) and container creation overhead.
- Docker socket access (`/var/run/docker.sock`) needed by the Aira Go process, which is a privilege escalation vector if not properly secured.
- Container image management (building, tagging, distributing agent runtime images) adds operational complexity.

### Neutral

- Docker is already a standard dependency in most deployment environments, so the requirement is unlikely to be a blocking constraint.
- The interface abstraction adds a small amount of indirection but is justified by the multi-framework requirement.

## References

- [Docker Engine SDK for Go](https://docs.docker.com/engine/api/sdk/)
- [Agent Communication Protocol (ACP)](https://agentcommunicationprotocol.dev/)
- [Claude Code SDK](https://docs.anthropic.com/en/docs/claude-code)
