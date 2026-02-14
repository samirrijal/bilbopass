package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Cache implements ports.CacheService using Valkey (Redis-compatible).
type Cache struct {
	client valkey.Client
}

// New creates a new Valkey cache client.
func New(addr string) (*Cache, error) {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{addr},
	})
	if err != nil {
		return nil, fmt.Errorf("valkey connect: %w", err)
	}
	return &Cache{client: client}, nil
}

// Get retrieves a value by key.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	cmd := c.client.Do(ctx, c.client.B().Get().Key(key).Build())
	if cmd.Error() != nil {
		return nil, cmd.Error()
	}
	b, err := cmd.AsBytes()
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Set stores a value with a TTL in seconds.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	cmd := c.client.Do(ctx,
		c.client.B().Set().Key(key).Value(string(value)).Ex(time.Duration(ttlSeconds)*time.Second).Build(),
	)
	return cmd.Error()
}

// Delete removes a key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	cmd := c.client.Do(ctx, c.client.B().Del().Key(key).Build())
	return cmd.Error()
}

// Close releases the client.
func (c *Cache) Close() {
	c.client.Close()
}
