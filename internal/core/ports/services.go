package ports

import (
	"context"

	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// EventPublisher publishes domain events to a message broker.
type EventPublisher interface {
	PublishVehiclePosition(ctx context.Context, vp *domain.VehiclePosition) error
	PublishDelayEvent(ctx context.Context, event *domain.DelayEvent) error
	PublishDetourAlert(ctx context.Context, tripID string) error
	PublishBroadcast(ctx context.Context, data []byte) error
}

// EventSubscriber subscribes to domain events from a message broker.
type EventSubscriber interface {
	SubscribeVehiclePositions(ctx context.Context, handler func(ctx context.Context, vp *domain.VehiclePosition) error) error
	SubscribeDelayEvents(ctx context.Context, handler func(ctx context.Context, event *domain.DelayEvent) error) error
	SubscribeDetourAlerts(ctx context.Context, handler func(ctx context.Context, tripID string) error) error
}

// CacheService provides read-through caching.
type CacheService interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttlSeconds int) error
	Delete(ctx context.Context, key string) error
}

// NotificationService sends notifications (push, email, etc.).
type NotificationService interface {
	SendPush(ctx context.Context, userID, title, body string) error
}
