// Package pubsub publishes alert events to a message system.
// Publisher is an interface so other backends (for example Kafka) can be added.
package pubsub

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Publisher interface {
	Publish(ctx context.Context, channel string, payload []byte) error
}

// Noop is used when no message system is configured.
type Noop struct{}

func (Noop) Publish(context.Context, string, []byte) error { return nil }

type Redis struct {
	client *redis.Client
}

func NewRedis(addr string) *Redis {
	return &Redis{client: redis.NewClient(&redis.Options{Addr: addr})}
}

func (r *Redis) Publish(ctx context.Context, channel string, payload []byte) error {
	return r.client.Publish(ctx, channel, payload).Err()
}
