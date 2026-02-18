# AGENTS.md — Aira (gosuda/aira)

AI-native kanban platform. Go 1.25+ backend, SvelteKit frontend, Docker-isolated agent execution.

Module: `github.com/gosuda/aira` · Toolchain: `go1.26.0` · CGO: disabled (`CGO_ENABLED=0`)

---

## Project Architecture

```
cmd/aira/           → Entry point (zerolog init, config, server start)
internal/
  config/           → Env-based config (DatabaseConfig, RedisConfig, JWTConfig, etc.)
  domain/           → Entities (User, Tenant, Project, Task, ADR, AgentSession, HITL)
  auth/             → JWT + OAuth2 + API key + account linking
  server/           → chi v5 router + Huma v2 API + middleware
  server/middleware/ → Auth (JWT/API key), tenant extraction, rate limiting
  api/v1/           → Huma REST endpoints (auth, boards, tasks, ADRs, agents, projects, tenants)
  api/ws/           → WebSocket hub (board + agent streams via Redis pub/sub)
  agent/            → AgentBackend interface, Registry, Orchestrator, Docker runtime
  agent/backends/   → Claude, Codex, OpenCode, ACP transport implementations
  messenger/        → Messenger interface + HITL router
  messenger/slack/  → Slack Events API + Block Kit
  messenger/discord/→ Discord bot integration
  messenger/telegram/→ Telegram bot integration
  store/postgres/   → PostgreSQL repos (pgx v5), all tenant-scoped
  store/redis/      → Redis pub/sub (board/agent channels) + cache
  notify/           → Push notification dispatcher
  enterprise/       → RBAC, OAuth SSO, encrypted secrets
  secrets/          → Secret encryption service
web/                → SvelteKit 5 dashboard (kanban + ADR timeline + agent monitor)
migrations/         → PostgreSQL migration files
docs/adrs/          → 9 MADR architectural decision records (ADR-0001 through ADR-0009)
```

---

## Key Interfaces

```go
// AgentBackend — universal AI agent integration (agent/backend.go)
type AgentBackend interface {
    StartSession(ctx context.Context, opts SessionOptions) (SessionID, error)
    SendPrompt(ctx context.Context, sessionID SessionID, prompt string) error
    Cancel(ctx context.Context, sessionID SessionID) error
    OnMessage(handler MessageHandler)
    Dispose(ctx context.Context) error
}

// TransportHandler — per-agent protocol adapter (agent/transport.go)
type TransportHandler interface {
    AgentName() string
    InitTimeout() time.Duration
    IdleTimeout() time.Duration
    FilterOutput(line string) (string, bool)
    ParseToolCall(raw json.RawMessage) (ToolCall, error)
}

// Messenger — platform-agnostic messenger operations (messenger/messenger.go)
type Messenger interface {
    SendMessage(ctx context.Context, channelID string, text string) (MessageID, error)
    CreateThread(ctx context.Context, channelID, parentID, text string, options []QuestionOption) (ThreadID, error)
    UpdateMessage(ctx context.Context, channelID, messageID, text string) error
    SendNotification(ctx context.Context, userExternalID, text string) error
    Platform() string
}
```

---

## Logging

Uses `github.com/rs/zerolog` (not stdlib slog). Import `"github.com/rs/zerolog/log"`.

```go
log.Info().Str("addr", addr).Msg("starting server")
log.Error().Err(err).Str("session_id", id).Msg("agent failed")
log.Warn().Str("user_id", uid).Msg("no messenger links")
```

---

## Formatting & Style

**Mandatory** before every commit: `gofmt -w . && goimports -w .`

Import ordering: **stdlib → external → internal** (blank-line separated). Local prefix: `github.com/gosuda`.

**Naming:** packages lowercase single-word (`httpwrap`) · interfaces as behavior verbs (`Reader`, `Handler`) · errors `Err` prefix sentinels (`ErrNotFound`), `Error` suffix types · context always first param `func Do(ctx context.Context, ...)`

**CGo:** always disabled — `CGO_ENABLED=0`. Pure Go only. No C dependencies.

---

## Static Analysis & Linters

| Tool | Command |
|------|---------|
| Built-in vet | `go vet ./...` |
| golangci-lint v2 | `golangci-lint run` |
| Race detector | `go test -race ./...` |
| Vulnerability scan | `govulncheck ./...` |

Full configuration: **[`.golangci.yml`](.golangci.yml)**. Linter tiers:

- **Correctness** — `govet`, `errcheck`, `staticcheck`, `unused`, `gosec`, `errorlint`, `nilerr`, `copyloopvar`, `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `durationcheck`, `makezero`, `noctx`
- **Quality** — `gocritic` (all tags), `revive`, `unconvert`, `unparam`, `wastedassign`, `misspell`, `whitespace`, `godot`, `goconst`, `dupword`, `usestdlibvars`, `testifylint`, `testableexamples`, `tparallel`, `usetesting`
- **Concurrency safety** — `gochecknoglobals`, `gochecknoinits`, `containedctx`
- **Performance & modernization** — `prealloc`, `intrange`, `modernize`, `fatcontext`, `perfsprint`, `reassign`, `spancheck`, `mirror`, `recvcheck`

---

## Error Handling

1. **Wrap with `%w`** — always add call-site context: `return fmt.Errorf("repo.Find: %w", err)`
2. **Sentinel errors** per package with domain prefix: `var ErrNotFound = errors.New("domain: not found")`
3. **Multi-error** — use `errors.Join(err1, err2)` or `fmt.Errorf("op: %w and %w", e1, e2)`
4. **Never ignore errors** — `_ = fn()` only for `errcheck.exclude-functions`
5. **Fail fast** — return immediately; no state accumulation after failure
6. **Check with `errors.Is`/`errors.As`** — never string-match `err.Error()`

---

## Iterators (Go 1.23+)

Signatures: `func(yield func() bool)` · `func(yield func(V) bool)` · `func(yield func(K, V) bool)`

**Rules:** always check yield return (panics on break if ignored) · avoid defer/recover in iterator bodies · use stdlib (`slices.All`, `slices.Backward`, `slices.Collect`, `maps.Keys`, `maps.Values`) · range over integers: `for i := range n {}`

---

## Context & Concurrency

Every public I/O function **must** take `context.Context` first.

| Pattern | Primitive |
|---------|-----------|
| Parallel work with errors | `errgroup.Group` (preferred over `WaitGroup`) |
| Bounded concurrency | `errgroup.SetLimit` or buffered channel semaphore |
| Fan-out/fan-in | Unbuffered chan + N producers + 1 consumer; `select` to merge |
| Pipeline stages | `chan T` between stages, sender closes to signal done |
| Cancellation/timeout | `context.WithCancel` / `context.WithTimeout` |
| Concurrent read/write | `sync.RWMutex` (encapsulate behind methods) |
| Lock-free counters | `atomic.Int64` / `atomic.Uint64` |
| One-time init | `sync.Once` / `sync.OnceValue` / `sync.OnceFunc` |
| Object reuse | `sync.Pool` (hot paths only, no lifetime guarantees) |

**Goroutine rules:** creator owns lifecycle (start, stop, errors, panic recovery) · no bare `go func()` · every goroutine needs a clear exit (context, done channel, bounded work) · leaks are bugs — verify with `goleak` or `runtime.NumGoroutine()`

**Channel rules:** use directional types (`chan<-`/`<-chan`) in signatures · only sender closes · nil channel blocks forever (use to disable `select` cases) · unbuffered = synchronization, buffered = decoupling/backpressure · `for v := range ch` until closed · `select` with `default` only for non-blocking try-send/try-receive

**Select patterns:** timeout via `context.WithTimeout` (not `time.After` in loops — leaks timers) · always check `ctx.Done()` · fan-in merges with multi-case `select` · rate-limit with `time.Ticker` not `time.Sleep`

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(maxWorkers)
for _, item := range items {
    g.Go(func() error { return process(ctx, item) })
}
if err := g.Wait(); err != nil { return fmt.Errorf("processAll: %w", err) }
```

**Anti-patterns:** shared memory without sync · `sync.Mutex` in public APIs · goroutine without context · closing channel from receiver · sending on closed channel · `time.Sleep` for synchronization · unbounded goroutine spawn

---

## Testing

```bash
go test -v -race -coverprofile=coverage.out ./...
```

- **Benchmarks (Go 1.24+):** `for b.Loop() {}` — prevents compiler opts, excludes setup from timing
- **Test contexts (Go 1.24+):** `ctx := t.Context()` — auto-canceled when test ends
- **Table-driven tests** as default · **race detection** (`-race`) mandatory in CI
- **Fuzz testing:** `go test -fuzz=. -fuzztime=30s` — fast, deterministic targets
- **testify** for assertions when stdlib `testing` is verbose

---

## Database

PostgreSQL via `pgx/v5`. All queries **must** be tenant-scoped (tenant_id in WHERE clause). Repository pattern enforces this via interface methods that require `tenantID uuid.UUID`.

Key tables: `tenants`, `users`, `projects`, `adrs`, `tasks`, `agent_sessions`, `hitl_questions`, `messenger_connections`, `repo_volumes`, `audit_log`

Migrations in `migrations/` — applied via `make migrate-up`, rolled back via `make migrate-down` (dev/test only).

---

## Agent Execution

- Each agent session runs in an isolated Docker container with persistent repo volume
- Branch isolation: agent works on `aira/<session-id>` branch, merges/PRs on completion
- 4 backends: Claude SDK, Codex, OpenCode, ACP (via `AgentRegistry`)
- HITL routing: agent questions → messenger thread → human reply → agent resumes
- ADR creation: agents call `create_adr` tool + post-processing extracts implicit decisions

---

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/danielgtaylor/huma/v2` | OpenAPI 3.1 generation |
| `github.com/coder/websocket` | WebSocket (net/http native) |
| `github.com/jackc/pgx/v5` | PostgreSQL driver |
| `github.com/redis/go-redis/v9` | Redis client |
| `github.com/golang-jwt/jwt/v5` | JWT auth |
| `github.com/rs/zerolog` | Structured logging |
| `github.com/rs/cors` | CORS middleware |
| `github.com/docker/docker` | Docker Engine API |
| `github.com/slack-go/slack` | Slack API client |
| `golang.org/x/crypto` | argon2id password hashing |

---

## Security

- **Vulnerability scanning:** `govulncheck ./...` — CI and pre-release
- **Module integrity:** `go mod verify` — validates checksums against go.sum
- **Supply chain:** always commit `go.sum` · audit with `go mod graph` · pin toolchain
- **Crypto:** FIPS 140-3, post-quantum X25519MLKEM768, `crypto/rand.Text()` for secure tokens
- **G117 nolint:** DTO/config fields holding secrets use `//nolint:gosec` directives

---

## Performance

- **Object reuse:** `sync.Pool` hot paths · `weak.Make` for cache-friendly patterns
- **Benchmarking:** `go test -bench=. -benchmem` · `-cpuprofile`/`-memprofile`
- **Avoid `reflect`:** prefer generics, type switches, interfaces, or `go generate`
- **PGO:** production CPU profile → `default.pgo` in main package → rebuild (2-14% gain)
- **GOGC:** default 100; high-throughput `200-400`; memory-constrained `GOMEMLIMIT` + `GOGC=off`

---

## Module Hygiene

- **Always commit** `go.mod` and `go.sum` · **never commit** `go.work`
- **Pin toolchain:** `toolchain go1.26.0` in go.mod
- **Tool directive (Go 1.24+):** `tool golang.org/x/tools/cmd/stringer` in go.mod
- **Pre-release:** `go mod tidy && go mod verify && govulncheck ./...`

---

## CI/CD & Tooling

| File | Purpose |
|------|---------|
| [`.golangci.yml`](.golangci.yml) | golangci-lint v2 configuration |
| [`Makefile`](Makefile) | Build/lint/test/vuln/migrate targets |
| [`.github/workflows/ci.yml`](.github/workflows/ci.yml) | GitHub Actions: test → lint → security → build |
| [`Dockerfile`](Dockerfile) | Multi-stage build (Go + SvelteKit → scratch) |
| [`docker-compose.yml`](docker-compose.yml) | Dev: PostgreSQL + Redis + Aira |

**Pre-commit:** `make all` or `gofmt -w . && goimports -w . && go vet ./... && golangci-lint run && go test -race ./... && govulncheck ./...`

---

## Verbalized Sampling

Before trivial or non-trivial changes, AI agents **must**:

1. **Sample 3-5 intent hypotheses** — rank by likelihood, note one weakness each
2. **Explore edge cases** — up to 3 standard, 5 for architectural changes
3. **Assess coupling** — structural (imports), temporal (co-changing files), semantic (shared concepts)
4. **Tidy first** — high coupling → extract/split/rename before changing; low → change directly
5. **Surface decisions** — ask the human when trade-offs exist; do exactly what is asked, no more
