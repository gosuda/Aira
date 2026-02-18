# ADR-0005: Slack-First Messenger Integration with Abstraction Layer

## Status

Accepted

## Date

2026-02-17

## Context

Aira supports Human-in-the-Loop (HITL) workflows where AI agents can pause execution to ask humans questions, request approvals, or surface decisions for review. These interactions must reach humans where they already work: Slack, Discord, and Telegram. The system cannot be locked to a single messenger platform, but building all three integrations simultaneously is impractical for MVP.

Key forces:

- **HITL routing** must deliver agent questions to the correct human in their preferred messenger platform and collect responses back to the agent.
- **Platform-specific features** (Slack Block Kit interactive components, Discord embeds and reactions, Telegram inline keyboards) provide significantly better UX than plain text and should not be sacrificed for abstraction purity.
- **Webhook handling** differs fundamentally between platforms: Slack uses Events API with HMAC verification, Discord uses Gateway WebSocket, Telegram uses Bot API webhooks with token-based auth.
- **Thread semantics** vary: Slack has native message threads, Discord has forum channels and thread creation, Telegram has reply-to chains.
- **MVP scope** requires Slack first, with Discord and Telegram following.

## Decision

Define a **`Messenger` interface** that abstracts cross-platform messaging operations:

```go
type Messenger interface {
    SendMessage(ctx context.Context, channel string, msg Message) (MessageID, error)
    CreateThread(ctx context.Context, channel string, parentID MessageID, msg Message) (ThreadID, error)
    UpdateMessage(ctx context.Context, channel string, msgID MessageID, msg Message) error
    SendNotification(ctx context.Context, userID string, msg Message) error
    Platform() Platform
}
```

- **`SlackMessenger`** is the first implementation, using the Slack Web API for message operations and Slack Events API for incoming webhooks.
- **HITL Router** uses the `Messenger` interface to route agent questions to the appropriate platform based on the user's linked messenger account (see ADR-0008). The router does not know which platform it is communicating with.
- **Platform-specific webhook handlers** are registered as separate HTTP handlers (not behind the `Messenger` interface) because each platform's webhook verification and event parsing is fundamentally different.
- **`Message` struct** uses a platform-neutral format (text, structured fields, action buttons) that each `Messenger` implementation translates to platform-native formatting (Slack Block Kit, Discord embeds, Telegram Markdown).
- Discord and Telegram implementations follow the same interface contract when added.

## Alternatives Considered

### Alternative 1: Direct Slack API calls throughout codebase

- Call the Slack Web API directly from HITL routing, notification, and agent interaction code without an abstraction layer.
- **Rejected because:** Every callsite becomes Slack-specific, making it painful to add Discord or Telegram support later. Business logic (HITL routing decisions, notification preferences) becomes entangled with platform-specific API details. Testing requires mocking Slack API responses at every callsite rather than at one interface boundary.

### Alternative 2: Universal bot framework (e.g., go-chat-bot)

- Use a generic multi-platform bot framework that provides a unified API across messengers.
- **Rejected because:** Generic frameworks use a lowest-common-denominator message format that sacrifices Slack's interactive Block Kit components, Discord's rich embeds, and Telegram's inline keyboards. These platform-specific features are essential for good HITL UX (e.g., approve/reject buttons, structured decision summaries). The framework would add a dependency that may not keep up with platform API changes.

## Consequences

### Positive

- Business logic (HITL routing, notification delivery, agent interaction) is messenger-agnostic: it depends only on the `Messenger` interface.
- Adding a new messenger platform requires implementing one interface and registering a webhook handler. No changes to HITL routing or agent interaction logic.
- Platform-specific features (Slack Block Kit, Discord embeds) are preserved in the implementation layer, providing native UX quality on each platform.
- Testing business logic requires only a mock `Messenger` implementation, not platform-specific API mocks.

### Negative

- The `Message` struct must be a lowest-common-denominator format that maps to all platforms. Some platform-specific features (e.g., Slack's `overflow` menu, Discord's `select` components) may not have equivalents and require platform-specific extensions.
- Thread semantics differ between platforms: Slack's threaded replies, Discord's forum threads, and Telegram's reply chains have different behaviors around notification, visibility, and nesting depth. The `CreateThread` abstraction may not capture all platform nuances.
- Maintaining webhook handlers for each platform (Slack Events API, Discord Gateway, Telegram Bot API) is separate work from the `Messenger` implementation and cannot be shared.

### Neutral

- The `Messenger` interface intentionally does not abstract webhook handling because the ingress patterns are too different to unify meaningfully.
- Platform priority (Slack first, Discord second, Telegram third) is driven by target user demographics, not technical constraints.

## References

- [Slack Block Kit](https://api.slack.com/block-kit)
- [Slack Events API](https://api.slack.com/events-api)
- [Discord Gateway](https://discord.com/developers/docs/topics/gateway)
- [Telegram Bot API](https://core.telegram.org/bots/api)
- ADR-0008: Standalone Auth with Messenger Account Linking
