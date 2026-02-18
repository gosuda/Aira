# ADR-0001: Use Huma v2 with chi v5 as HTTP Framework

## Status

Accepted

## Date

2026-02-17

## Context

Aira is a multi-tenant AI-native kanban SaaS that requires a robust HTTP framework capable of supporting REST APIs, WebSocket connections, Slack webhook ingestion, and automatic OpenAPI specification generation. The framework must be compatible with Go's `net/http` standard library to allow unrestricted middleware composition and must not impose protocol constraints that conflict with raw HTTP endpoints (e.g., Slack Events API verification, WebSocket upgrade).

Key forces at play:

- **OpenAPI 3.1 generation** must be automatic from Go types to avoid spec drift and manual maintenance.
- **RFC 9457 (Problem Details)** error responses are required for consistent API error formatting.
- **WebSocket support** via `nhooyr.io/websocket` demands raw `http.Handler` access, not framework-wrapped handlers.
- **Slack webhooks** require raw request body access for HMAC signature verification before any framework parsing occurs.
- **Middleware ecosystem** must be `net/http`-compatible to leverage existing Go middleware (CORS, rate limiting, auth).

## Decision

Use **Huma v2** as the API layer on top of **chi v5** as the HTTP router.

- **Huma v2** provides automatic OpenAPI 3.1 spec generation from Go struct tags and type definitions, built-in RFC 9457 error responses, and request input validation. Huma's BYOIR (Bring Your Own Input/Output Resolver) architecture means the API layer can be replaced without rewriting route handlers.
- **Chi v5** provides a lightweight, idiomatic `net/http`-compatible router with middleware chaining. Chi routes are standard `http.Handler`, enabling raw handler registration for WebSocket endpoints and Slack webhook receivers.
- **WebSocket endpoints** are registered as raw chi routes using `nhooyr.io/websocket`, bypassing Huma's request/response abstraction.
- **Slack webhook handlers** are registered as raw chi handlers to access the raw request body for HMAC-SHA256 signature verification before any parsing.

## Alternatives Considered

### Alternative 1: Fiber v3

- High-performance HTTP framework built on FastHTTP with Express-like API.
- **Rejected because:** FastHTTP is not `net/http`-compatible, meaning standard Go middleware cannot be used. FastHTTP's `RequestCtx` is reused across requests, making it unsafe for concurrent access patterns common in WebSocket and streaming handlers. The ecosystem lock-in to FastHTTP-specific middleware is unacceptable for long-term maintainability.

### Alternative 2: Connect-go (Buf)

- Modern RPC framework supporting gRPC, gRPC-Web, and Connect protocols with Protobuf-first API definitions.
- **Rejected because:** Protobuf-first API design introduces unnecessary friction for REST endpoints, Slack webhook handling, and browser-facing APIs. Slack Events API sends JSON with HMAC signatures that do not map cleanly to Protobuf service definitions. The additional Protobuf compilation step and code generation toolchain add build complexity without proportional benefit for a REST-primary API.

### Alternative 3: Echo v5

- Mature HTTP framework with middleware support, request binding, and OpenAPI generation via plugins.
- **Rejected because:** Echo v5's API remains unstable as of 2026-02, with the release timeline pushed to 2026-03-31. Building on an unreleased API introduces risk of breaking changes during development. Echo's OpenAPI generation is plugin-based rather than type-driven, producing less reliable spec output compared to Huma's native approach.

### Alternative 4: Standard library only (net/http + ServeMux)

- Go 1.22+ enhanced `ServeMux` with method-based routing and path parameters.
- **Rejected because:** No built-in OpenAPI generation requires manual spec maintenance, which inevitably drifts from implementation. Request validation, error formatting, and input parsing would all need to be hand-rolled, increasing boilerplate and inconsistency across endpoints. Chi provides the same `net/http` compatibility with significantly less boilerplate.

## Consequences

### Positive

- Automatic OpenAPI 3.1 specification generated directly from Go types eliminates spec drift.
- RFC 9457 error responses are consistent across all API endpoints without manual formatting.
- Full `net/http` compatibility means any Go middleware works without adaptation.
- BYOIR architecture allows swapping Huma's API layer without rewriting chi route definitions or middleware.
- WebSocket and Slack webhook handlers operate as raw `http.Handler` with no framework interference.

### Negative

- Huma has a moderate bus-factor risk as a single-maintainer project; if the maintainer becomes inactive, the project may stagnate.
- Huma's abstraction layer (operation registration, input/output types, middleware adapter) introduces a learning curve for developers unfamiliar with its conventions.
- Two layers (Huma + chi) means developers must understand which layer handles what: chi for routing and middleware, Huma for API operations and OpenAPI generation.

### Neutral

- Chi v5 is widely adopted in the Go ecosystem and is unlikely to require migration.
- Huma's adapter pattern means a future migration to a different API layer (e.g., stdlib-only) is feasible without rewriting business logic.

## References

- [Huma v2 repository](https://github.com/danielgtaylor/huma)
- [Chi v5 repository](https://github.com/go-chi/chi)
- [nhooyr.io/websocket](https://github.com/coder/websocket)
- [RFC 9457 - Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457)
- [OpenAPI 3.1 Specification](https://spec.openapis.org/oas/v3.1.0)
