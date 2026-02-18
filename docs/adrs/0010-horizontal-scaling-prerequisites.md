# ADR-0010: In-Process State Stores and Horizontal Scaling Path

## Status

Accepted

## Date

2026-02-18

## Context

The 12-Factor App compliance audit (Factor VI: Processes) identified three in-process state stores that prevent horizontal scaling beyond a single instance. These are acceptable for the current single-instance MVP deployment but must be externalized before running multiple replicas behind a load balancer.

The three state stores are:

1. **IP Rate Limiter** (`internal/server/middleware/ratelimit.go`) — A `sync.RWMutex`-guarded map (`ipLimiter`) that tracks per-IP request counts for rate limiting. Each instance maintains its own counter, so N instances effectively multiply the allowed rate by N.

2. **Orchestrator Backends Map** (`internal/agent/orchestrator.go`) — A `sync.Mutex`-guarded map (`backends`) tracking active agent session backends. On restart, active sessions recorded in PostgreSQL have no corresponding in-memory backend, causing orphaned containers.

3. **OAuth Pending Links** (`internal/auth/oauth.go`) — A `sync.Map` (`pendingLinks`) storing OAuth state tokens during the authorization flow. If the callback hits a different instance than the one that initiated the flow, the state token lookup fails and the OAuth flow breaks.

## Decision

Document the upgrade path for each state store. No code changes at this time — the current single-instance deployment is correct for the MVP phase. When horizontal scaling is required, implement the following migrations in priority order:

1. **IP Rate Limiter -> Redis**: Replace the in-memory map with Redis `INCR` + `EXPIRE` per IP key. Use a sliding window algorithm (e.g., Redis sorted sets with timestamps). This is the highest priority because incorrect rate limiting is a security concern.

2. **OAuth Pending Links -> Redis with TTL**: Store OAuth state tokens in Redis with a 10-minute TTL instead of `sync.Map`. The existing cleanup goroutine can be removed since Redis handles expiration natively. This is medium priority because OAuth flows are user-facing and break visibly.

3. **Orchestrator Backends -> Database Reconciliation**: On startup, query `agent_sessions` for sessions with status `running`, attempt to reconnect to their Docker containers, and clean up sessions whose containers no longer exist. This is lower priority because it only affects recovery after restart, not steady-state operation.

## Alternatives Considered

### Alternative 1: Externalize All State Stores Now

- Migrate all three stores to Redis/PostgreSQL immediately.
- **Rejected because:** Premature optimization. The MVP runs as a single instance. Adding Redis dependencies for rate limiting and OAuth state increases operational complexity without current benefit. The code paths are well-isolated and can be migrated independently when horizontal scaling is actually needed.

### Alternative 2: Use Sticky Sessions

- Configure load balancer session affinity to route requests from the same client to the same instance.
- **Rejected because:** Sticky sessions violate 12-Factor App principles (Factor VI) and create uneven load distribution. They also don't solve the orchestrator recovery problem (a restarted instance loses all in-memory backends regardless of session affinity). Sticky sessions are a workaround, not a solution.

### Alternative 3: Shared Memory via IPC

- Use shared memory or IPC mechanisms between instances.
- **Rejected because:** Platform-specific, adds deployment complexity, and doesn't work across hosts in a distributed deployment. Redis provides the same shared-state semantics with better tooling, monitoring, and operational familiarity.

## Consequences

### Positive

- Clear upgrade path documented before it becomes urgent.
- Each state store can be migrated independently without coupling.
- Priority ordering ensures security-critical changes (rate limiting) happen first.
- No premature complexity added to the MVP.

### Negative

- Horizontal scaling requires implementation work before it can be enabled.
- Developers must be aware of these constraints when designing new features that might add in-process state.

### Neutral

- Redis is already a project dependency (used for pub/sub), so the rate limiter and OAuth migrations don't introduce new infrastructure.
- The orchestrator reconciliation pattern is common in container orchestration systems and well-understood.

## References

- [The Twelve-Factor App - Factor VI: Processes](https://12factor.net/processes)
- [Redis Rate Limiting Patterns](https://redis.io/docs/latest/develop/use/patterns/rate-limiter/)
