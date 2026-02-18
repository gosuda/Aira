# ADR-0008: Standalone Auth with Messenger Account Linking

## Status

Accepted

## Date

2026-02-17

## Context

Aira has two primary interaction surfaces that require authentication:

1. **Web dashboard and REST API:** Users manage projects, review ADRs, interact with the kanban board, and configure agent settings.
2. **Messenger platforms (Slack, Discord, Telegram):** Users receive HITL questions from agents, approve/reject decisions, and monitor task progress.

These surfaces must share a unified identity model so that when an agent asks a question via the HITL router, the system can route it to the correct user on their preferred messenger platform and attribute the response back to their Aira identity.

Key forces:

- **Web dashboard auth** needs standard email/password and OAuth2 flows for low-friction onboarding.
- **Messenger identity linking** must connect a Slack/Discord/Telegram account to an Aira user account for HITL routing.
- **API access** for CI/CD pipelines and programmatic clients requires non-interactive authentication (API keys).
- **Self-hosted deployments** should work without external OAuth2 providers (Google, GitHub) for air-gapped environments.
- **Multi-tenant JWT tokens** must carry tenant claims for row-level data isolation (ADR-0004).

## Decision

Implement a **standalone authentication system** with messenger account linking:

### Primary auth (web dashboard + API)

- **Email/password:** Registration and login with passwords hashed using argon2id (memory-hard, GPU-resistant).
- **OAuth2:** Google and GitHub as identity providers for one-click sign-up/sign-in. OAuth2 flow creates or links an Aira account based on email matching.
- **JWT tokens:** Access tokens (short-lived, 15 minutes) and refresh tokens (long-lived, 7 days) for API authentication. JWT payload includes `user_id`, `tenant_id`, `roles`, and `permissions`. Tenant claims enable row-level isolation per ADR-0004.
- **Token refresh:** Refresh tokens are rotated on use (one-time use) and stored hashed in the database.

### API keys (programmatic access)

- Users generate API keys from the web dashboard for CI/CD integration and programmatic API access.
- Keys are SHA-256 hashed before storage; only the prefix is stored in plaintext for identification (e.g., `aira_sk_abc123...`).
- API keys inherit the generating user's tenant scope and permissions.

### Messenger account linking

- When a user first interacts with Aira on a messenger platform (e.g., replies to a HITL question in Slack), the bot sends a DM with a link to create or link their Aira account.
- The linking flow: user clicks link -> authenticates on web dashboard -> confirms account link -> messenger identity is associated with Aira user record.
- Multiple messenger accounts can be linked to a single Aira account (e.g., Slack for work, Discord for open-source projects).
- HITL router uses the linked messenger identity to deliver agent questions to the correct user on their preferred platform.

### Self-hosted mode

- Email/password auth is always available.
- OAuth2 providers are optional and can be disabled via configuration.
- A single admin account is auto-created at first startup with credentials from environment variables.
- API keys work identically in self-hosted mode.

## Alternatives Considered

### Alternative 1: Slack-only auth (Slack as identity provider)

- Use Slack's "Sign in with Slack" OAuth2 flow as the sole authentication method, leveraging Slack workspace membership as the identity source.
- **Rejected because:** Blocks access to the web dashboard for users without Slack accounts. Blocks self-hosted deployments in organizations that do not use Slack. Creates a hard dependency on Slack's OAuth2 availability. Does not support Discord or Telegram users who may not have Slack accounts.

### Alternative 2: API key only (no user identity)

- Use API keys as the sole authentication mechanism for both web dashboard and programmatic access.
- **Rejected because:** API keys have no user identity, making HITL routing impossible (cannot determine which human to ask). No support for web dashboard login flows (users cannot "sign in" with an API key in a browser). No OAuth2 means higher friction onboarding. No per-user permissions or audit trail.

## Consequences

### Positive

- Platform-independent authentication: users can access Aira regardless of which messenger platforms they use.
- Messenger account linking enables precise HITL routing: the system knows which Slack/Discord/Telegram account belongs to which Aira user.
- API keys enable CI/CD integration and programmatic access without sharing user credentials.
- OAuth2 (Google, GitHub) reduces onboarding friction for SaaS users.
- Self-hosted mode works without external identity providers, supporting air-gapped deployments.

### Negative

- More complex auth infrastructure than a single-provider approach: email/password, OAuth2, JWT, API keys, and messenger linking are all separate subsystems.
- Account linking flow required per messenger platform: each platform has different OAuth2/bot interaction patterns for identity verification.
- Token refresh handling (rotation, revocation, concurrent refresh race conditions) adds implementation complexity.
- JWT token size increases with tenant claims and permissions, which may matter for high-frequency API calls.

### Neutral

- argon2id is the current best practice for password hashing but is computationally expensive; server sizing must account for auth endpoint CPU usage under load.
- The messenger linking flow is a one-time action per user per platform, so UX friction is amortized.

## References

- [argon2id (OWASP Password Storage Cheat Sheet)](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [RFC 6749 - OAuth 2.0 Authorization Framework](https://www.rfc-editor.org/rfc/rfc6749)
- [RFC 7519 - JSON Web Token (JWT)](https://www.rfc-editor.org/rfc/rfc7519)
- ADR-0004: Multi-Tenant Architecture with Self-Hosted Support
- ADR-0005: Slack-First Messenger Integration with Abstraction Layer
