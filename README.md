# Aira-ai

**The open-source, agent-native engineering platform — the true AI-native evolution of Jira.**

One command from Slack, Discord, Telegram or any chat app and AI agents read your codebase, edit code, add features, run tests and deploy to production.  
Humans stay in the loop — faster, easier and more effective than ever with **live push notifications**.

---

## Vision

The real bottleneck in development is not writing code — it is human communication and verification. Aira removes unnecessary friction while preserving true human-in-the-loop control. AI takes full responsibility for every detail of execution while humans handle only direction and exceptions, accelerated by live push notifications that make oversight instant and effortless. The result is 5–10× faster development velocity and the complete disappearance of burnout.

Aira is the open-source release of what Spotify, Meta, and Tesla already run internally. ([Spotify's Honk](https://news.ycombinator.com/item?id=46994003))

This is not merely a tool. It is the foundation of the next chapter in the developer ecosystem.

---


## Features

- **Architectural Decision Records in Kanban** — AI auto-generates ADRs from every major change and displays them in a beautiful, searchable Kanban board
- **Zero-friction commands** — `@aira fix the auth bug` or `@aira add user profile page` from any chat
- **Full agent-native execution** — code reading, editing, testing, CI, deployment
- **True Human-in-the-Loop inside messengers** — every AI `question` tool is automatically turned into an interactive thread in Slack/Discord/Telegram. You simply reply in the same chat.
- **Live push notifications** — optional alerts when you’re offline (summary + direct link back to the thread)
- **Multi-platform support** — Slack, Discord, Telegram, Linear, Jira, email, mobile
- **Native agent framework support** — Claude Agent SDK, OpenCode, Codex, and more
- **Enterprise-ready** — concurrency, state persistence, audit logs, rollback, secret management
- **Language & stack agnostic** — works with any codebase
- **Self-hosted or cloud** — Docker, Kubernetes, or one-click deploy

---

## How It Works

1. You send a natural-language command in any supported chat
2. Aira spins up specialized AI agents that:
   - Read the latest codebase
   - Plan changes (and record the architectural decision)
   - Edit files, run tests, and deploy
3. Every architectural decision is saved as an ADR and moved on the Kanban board automatically
4. When human input is required, the AI posts the precise question directly into the original chat thread (e.g. “Should we use Redis or Dragonfly for this cache?”)
5. You reply naturally in the same thread — the AI continues instantly
6. Live push notifications keep you in the loop even when you’re away

You only review the big picture and answer questions when they appear. Everything else — including the permanent record of why — is handled.

---

## Quick Start

```bash
git clone https://github.com/yourorg/aira.git
cd aira
cp .env.example .env
docker compose up -d
```

Invite `@aira` to your workspace and type:

```
@aira implement dark mode toggle and record the decision
```

Aira will execute, create the ADR, update the Kanban board, and ask any questions directly in the thread.

**Takes under 5 minutes to be productive.**

---

## Architecture

- **Agent Orchestration**: Pluggable (Claude Agent SDK native first)
- **ADR + Kanban Engine**: Built-in, versioned, searchable
- **HITL Mapping Layer**: Automatically converts any `question` tool call into messenger threads
- **State**: PostgreSQL + Redis
- **Notifications**: Native in-messenger push
- **Security**: Full audit trail of every decision and question

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

---

## Tech Stack

- Go 1.26+
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

## License

Apache 2.0 © 2026 Metaphorics

---

**Star this repo if you want every architectural decision to be remembered and development to move 10× faster — all inside the chats you already use.**

Made with ❤️ for teams who are tired of repeating the same mistakes and switching tabs.
