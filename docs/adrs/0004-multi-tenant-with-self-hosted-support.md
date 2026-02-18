# ADR-0004: Multi-Tenant Architecture with Self-Hosted Support

## Status

Accepted

## Date

2026-02-17

## Context

Aira must serve two deployment models from a single codebase:

1. **Multi-tenant SaaS:** Multiple organizations share the same Aira instance, each with isolated data, users, projects, and agent execution environments.
2. **Self-hosted single-tenant:** A single organization runs Aira on their own infrastructure for data sovereignty, compliance, or air-gapped environments.

Key forces:

- **Data isolation** between tenants is non-negotiable: one organization must never see another's projects, tasks, ADRs, or agent outputs.
- **Deployment simplicity** for self-hosted users who should not need to understand multi-tenancy concepts to run Aira.
- **Codebase maintainability** requires a single binary and shared code path to avoid feature drift between SaaS and self-hosted editions.
- **Database-level enforcement** is needed as a defense-in-depth measure beyond application-level tenant filtering.

## Decision

Implement **row-level tenant isolation** with a deployment mode toggle:

- **Every database table** includes a `tenant_id` column. All queries are scoped by `tenant_id` extracted from the authenticated user's JWT claims.
- **Middleware** extracts the tenant context from the JWT token on every request and injects it into `context.Context`. Repository-layer functions receive tenant-scoped contexts and apply the filter automatically.
- **Composite foreign keys** include `tenant_id` in all foreign key relationships (e.g., `tasks.tenant_id + tasks.project_id` references `projects.tenant_id + projects.id`). This provides database-level enforcement: a task cannot reference a project from a different tenant, even if application code has a bug.
- **Self-hosted mode** is activated via the `AIRA_SELF_HOSTED=true` environment variable. In this mode:
  - A single default tenant is auto-created at startup.
  - Auth is simplified (single admin account + API keys, no OAuth2 required).
  - SaaS-only features (billing, tenant management UI, usage metering) are disabled via feature flags.
  - All API requests are implicitly scoped to the default tenant.
- **Feature flags** control SaaS-only functionality, checked at the handler level to avoid conditional logic deep in business logic.

## Alternatives Considered

### Alternative 1: Schema-per-tenant (PostgreSQL schemas)

- Each tenant gets a dedicated PostgreSQL schema with identical table structures. Queries use `SET search_path` to scope to the correct tenant.
- **Rejected because:** Migration complexity scales linearly with tenant count: every schema migration must be applied to every tenant schema. Schema creation/deletion requires DDL privileges and cannot be done transactionally. Connection pooling becomes complicated when each connection may target a different schema. Backup and restore of individual tenants requires schema-level tooling.

### Alternative 2: Separate binaries for SaaS and self-hosted

- Build two binaries from the same codebase using build tags or separate `main` packages.
- **Rejected because:** Inevitably leads to feature drift as SaaS-specific code diverges from the self-hosted variant. Testing matrix doubles. Bug fixes must be verified in both build configurations. A single binary with runtime configuration is simpler to build, test, distribute, and support.

## Consequences

### Positive

- Single binary for all deployment modes simplifies CI/CD, distribution, and support.
- Row-level isolation is straightforward to implement and reason about: every query includes `WHERE tenant_id = ?`.
- Composite foreign keys provide database-level enforcement of tenant boundaries, catching application-level bugs before they become data leaks.
- Self-hosted mode is zero-configuration for single teams: set one environment variable and run.
- Feature flags keep SaaS-only logic at the edge (handlers), not buried in domain logic.

### Negative

- Every query must be tenant-scoped: a missing `tenant_id` filter is a cross-tenant data leak. Code review discipline and automated linting are required to catch missing filters.
- Composite foreign keys add complexity to schema definitions and migrations: all foreign key relationships must include `tenant_id`, increasing the number of columns in indexes.
- Query performance depends on `tenant_id` being in every relevant index, which increases index storage.

### Neutral

- Row-level security (RLS) via PostgreSQL policies could be added as an additional defense layer but is not implemented in the MVP to avoid debugging complexity with ORM-generated queries.
- The `AIRA_SELF_HOSTED` toggle is a compile-time-like switch at startup; it does not support runtime changes.

## References

- [PostgreSQL Row-Level Security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- ADR-0008: Standalone Auth with Messenger Account Linking (auth details for both modes)
