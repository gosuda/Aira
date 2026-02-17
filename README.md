# Aira

**The open-source, agent-native engineering platform — the true AI-native evolution of Jira.**

One command from Slack, Discord, Telegram or any chat app and AI agents read your codebase, edit code, add features, run tests and deploy to production.  
Humans stay in the loop — faster, easier and more effective than ever with **live push notifications**.

---

## Vision

The real bottleneck in development is not writing code — it is human communication and verification. Aira removes unnecessary friction while preserving true human-in-the-loop control. AI takes full responsibility for every detail of execution while humans handle only direction and exceptions, accelerated by live push notifications that make oversight instant and effortless. The result is 5–10× faster development velocity and the complete disappearance of burnout.

Aira is the open-source release of what Spotify, Meta, and Tesla already run internally.

**Fork it. Customize it. Grow it with us.**  
This is not merely a tool. It is the foundation of the next chapter in the developer ecosystem.

---

## Features

- **Zero-friction commands** — `@aira fix the auth bug` or `@aira add user profile page` from any chat
- **Full agent-native execution** — code reading, editing, testing, CI, deployment
- **True human-in-the-loop** — live push notifications with precise diffs, test results, and one-tap approve/rollback
- **Multi-platform support** — Slack, Discord, Telegram, Linear, Jira, email, mobile (iOS/Android)
- **Native agent framework support** — Claude Agent SDK, OpenCode, Codex, and more
- **Enterprise-ready** — concurrency, state persistence, audit logs, rollback, secret management
- **Language & stack agnostic** — works with any codebase (Go, TypeScript, Python, Rust, etc.)
- **Self-hosted or cloud** — Docker, Kubernetes, or one-click deploy

---

## How It Works

1. You send a natural-language command in any supported chat
2. Aira spins up specialized AI agents that:
   - Clone / read the latest codebase
   - Plan changes with architectural awareness
   - Edit files via agent SDK
   - Run tests locally and in CI
   - Generate PR or deploy directly (configurable)
3. You receive a **live push notification** with summary + key diffs + test status
4. One tap: Approve → merge & deploy / Reject → rollback
5. You only review the big picture. Everything else is handled.

---

## Quick Start

```bash
git clone https://github.com/yourorg/aira.git
cd aira
cp .env.example .env
# Edit .env with your Claude / OpenAI / Anthropic keys + GitHub / Slack tokens
docker compose up -d
```

Then invite `@aira` to your Slack/Discord/Telegram and type:

```
@aira implement dark mode toggle in the dashboard
```

Done. The AI will handle the rest and notify you when it’s ready for review.

**Takes under 5 minutes to be productive.**

---

## Architecture

- **Backend**: Go (high concurrency) or TypeScript (TypeScript-first teams) — your choice
- **Agent Orchestration Layer**: Pluggable (Claude Agent SDK native, OpenCode, Codex)
- **State & Persistence**: PostgreSQL + Redis
- **Notification Engine**: Unified push (Firebase, APNs, Slack DM, Telegram, email)
- **Security**: End-to-end encryption, least-privilege tools, full audit trail
- **Extensibility**: Plugin system for custom tools, new chat platforms, and deployment targets

---

## Supported Integrations

| Platform       | Status     | Notes                     |
|----------------|------------|---------------------------|
| Slack          | ✅ Native   | Commands + push           |
| Discord        | ✅ Native   | Commands + push           |
| Telegram       | ✅ Native   | Commands + push           |
| Linear         | ✅         | Issue → command sync      |
| Jira           | ✅         | Issue → command sync      |
| GitHub         | ✅         | PRs, commits, deploy      |
| GitLab / Bitbucket | ✅     | Full support              |
| Mobile Push    | ✅         | iOS & Android             |

---

## Tech Stack (Default)

- Go 1.23+ or TypeScript 5+
- Docker + Docker Compose
- PostgreSQL, Redis
- Claude Code with Agent SDK
- OpenCode / Codex Wrappers

---

## Roadmap

- [ ] Web dashboard (visual agent monitoring)
- [ ] Multi-repo & monorepo support
- [ ] Advanced planning agents (multi-step reasoning)
- [ ] Team role-based permissions
- [ ] On-prem enterprise edition
- [ ] Mobile app (iOS/Android)

---

## License

Apache 2.0 © 2026 Metaphorics
