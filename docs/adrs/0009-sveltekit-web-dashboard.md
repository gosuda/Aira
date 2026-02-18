# ADR-0009: SvelteKit Web Dashboard

## Status

Accepted

## Date

2026-02-17

## Context

Aira needs a web-based user interface for:

- **Kanban board** visualization with drag-and-drop task management across 4 states (Backlog, In Progress, In Review, Done).
- **ADR timeline** browsing with structured decision records linked to their derived tasks.
- **Agent monitoring** showing active agent sessions, real-time output streaming, and session history.
- **Project management** for creating and configuring projects, managing team members, and linking repositories.

The frontend must communicate with the Go backend via the REST API (OpenAPI-specified, per ADR-0001) and WebSocket connections for real-time updates. The deployment model must support both standalone service deployment (SaaS) and embedding into the Go binary for single-binary distribution (self-hosted, per ADR-0004).

Key forces:

- **Bundle size** matters for initial load performance, especially in self-hosted deployments where the frontend is served by the Go binary.
- **Real-time updates** for the kanban board require WebSocket integration with reactive state management.
- **Type safety** between frontend and backend is essential to prevent API contract drift.
- **Single-binary distribution** for self-hosted mode requires the frontend to be embeddable via `go:embed`.
- **Developer experience** should be productive for a small team without requiring deep framework expertise.

## Decision

Use **SvelteKit 5** with **Svelte 5 runes** for the web dashboard:

### Framework choice

- **SvelteKit 5** as the meta-framework for routing, layouts, and build configuration.
- **Svelte 5 runes** (`$state`, `$derived`, `$effect`) for reactive state management, replacing Svelte 4's implicit reactivity with explicit, fine-grained signals.
- **`@sveltejs/adapter-static`** for building the frontend as a static site (SPA) that can be embedded in the Go binary via `go:embed`.

### API communication

- **REST API client:** Generated from the backend's OpenAPI 3.1 spec using `openapi-typescript` (type generation) and `openapi-fetch` (runtime client). This guarantees type-safe API calls that match the backend's contract.
- **WebSocket:** Native WebSocket client for real-time kanban board updates, agent output streaming, and notification delivery. WebSocket messages use JSON with discriminated union types for type-safe message handling.

### Deployment modes

1. **Standalone service:** SvelteKit runs as its own Node.js process (or static files behind a CDN/reverse proxy) in SaaS deployments.
2. **Embedded in Go binary:** Static build output (`adapter-static`) is embedded into the Go binary via `go:embed` and served by the Go HTTP server. This enables single-binary distribution for self-hosted deployments.

## Alternatives Considered

### Alternative 1: Next.js (React)

- Industry-standard React meta-framework with SSR, ISR, and a large ecosystem.
- **Rejected because:** Larger bundle size than SvelteKit due to React runtime overhead. Server-side rendering is unnecessary for a dashboard application that requires authentication before rendering any content (no SEO benefit). React's virtual DOM diffing adds overhead for the frequent fine-grained updates needed by the kanban board and real-time agent output. The React + Next.js ecosystem is more complex than necessary for the scope of this dashboard.

### Alternative 2: Go templates + HTMX

- Server-rendered HTML with HTMX for dynamic updates, keeping the entire stack in Go.
- **Rejected because:** Drag-and-drop kanban board requires complex client-side state management (drag state, optimistic updates, reorder animations) that HTMX is not designed for. Real-time agent output streaming with rich formatting (code blocks, ANSI colors) exceeds what HTMX's swap-based model handles well. The limited interactivity model would result in a significantly worse user experience for the primary use cases.

### Alternative 3: Separate frontend repository

- Maintain the SvelteKit frontend in a separate Git repository from the Go backend.
- **Rejected because:** Coordination overhead between frontend and backend changes increases when API contracts change. OpenAPI type generation requires the spec from the backend repo. The `go:embed` integration for single-binary distribution requires the frontend build output to be present in the Go repository at build time. A monorepo with a `web/` directory is simpler to maintain.

## Consequences

### Positive

- Small bundle size: Svelte compiles components to imperative DOM updates with no framework runtime, resulting in significantly smaller bundles than React or Vue.
- Svelte 5 runes provide explicit, fine-grained reactivity that is easier to reason about than Svelte 4's implicit reactivity or React's hooks rules.
- Single-binary distribution via `go:embed` enables zero-dependency self-hosted deployments: one binary, one database, no Node.js required.
- Type-safe API client generated from OpenAPI spec prevents API contract drift between frontend and backend.
- WebSocket integration with Svelte's reactive system enables efficient real-time kanban board updates without manual DOM manipulation.

### Negative

- Two build systems (Go + Node.js/Vite) must be coordinated: the frontend must be built before the Go binary to embed static assets.
- Svelte's community and ecosystem is smaller than React's, meaning fewer third-party component libraries and fewer developers familiar with the framework.
- `go:embed` requires building the frontend before compiling the Go binary, adding a build step and making the Go build depend on Node.js tooling being available.
- SPA mode (via `adapter-static`) means no server-side rendering, which is acceptable for an authenticated dashboard but means the initial load shows a loading state.

### Neutral

- SvelteKit 5 with Svelte 5 runes is a major API change from Svelte 4. The team adopts the new API from the start, avoiding migration costs later.
- The `web/` directory in the monorepo is a standard SvelteKit project that can be developed and tested independently of the Go backend using mock API responses.

## References

- [SvelteKit documentation](https://svelte.dev/docs/kit)
- [Svelte 5 runes](https://svelte.dev/docs/svelte/$state)
- [openapi-typescript](https://openapi-ts.dev/)
- [openapi-fetch](https://openapi-ts.dev/openapi-fetch/)
- [`@sveltejs/adapter-static`](https://svelte.dev/docs/kit/adapter-static)
- [Go `embed` package](https://pkg.go.dev/embed)
- ADR-0001: Use Huma v2 with chi v5 as HTTP Framework (OpenAPI generation)
- ADR-0004: Multi-Tenant Architecture with Self-Hosted Support (self-hosted deployment)
