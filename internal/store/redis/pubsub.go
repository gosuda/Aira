package redis

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type PubSub struct {
	client *redis.Client
}

func New(ctx context.Context, addr, password string, db int) (*PubSub, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("redis.New: close after ping failure")
		}
		return nil, fmt.Errorf("redis.New: ping: %w", err)
	}

	return &PubSub{client: client}, nil
}

func (ps *PubSub) Close() error {
	if err := ps.client.Close(); err != nil {
		return fmt.Errorf("redis.PubSub.Close: %w", err)
	}
	return nil
}

func (ps *PubSub) Publish(ctx context.Context, channel string, payload []byte) error {
	if err := ps.client.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("redis.PubSub.Publish: %w", err)
	}
	return nil
}

func (ps *PubSub) Subscribe(ctx context.Context, channel string) (messages <-chan []byte, cleanup func(), err error) {
	sub := ps.client.Subscribe(ctx, channel)

	// Wait for subscription confirmation.
	_, err = sub.Receive(ctx)
	if err != nil {
		if closeErr := sub.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("redis.PubSub.Subscribe: close after receive failure")
		}
		return nil, nil, fmt.Errorf("redis.PubSub.Subscribe: receive confirmation: %w", err)
	}

	out := make(chan []byte, 64)
	redisCh := sub.Channel()

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-redisCh:
				if !ok {
					return
				}
				select {
				case out <- []byte(msg.Payload):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	cleanup = func() {
		if closeErr := sub.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("redis.PubSub.Subscribe: cleanup close")
		}
	}

	return out, cleanup, nil
}

// BoardChannel returns the Redis channel name for a project board.
func BoardChannel(tenantID, projectID uuid.UUID) string {
	return "board:" + tenantID.String() + ":" + projectID.String()
}

// AgentChannel returns the Redis channel name for an agent session.
func AgentChannel(sessionID uuid.UUID) string {
	return "agent:" + sessionID.String()
}

// TenantChannel returns the Redis channel name for tenant-wide events.
func TenantChannel(tenantID uuid.UUID) string {
	return "tenant:" + tenantID.String()
}
