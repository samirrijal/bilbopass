package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// Subscriber implements ports.EventSubscriber using NATS JetStream.
type Subscriber struct {
	conn *nats.Conn
	js   nats.JetStreamContext
	subs []*nats.Subscription
}

// NewSubscriber creates a subscriber sharing a NATS connection.
func NewSubscriber(url string) (*Subscriber, error) {
	conn, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := conn.JetStream()
	if err != nil {
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &Subscriber{conn: conn, js: js}, nil
}

func (s *Subscriber) SubscribeVehiclePositions(ctx context.Context, handler func(ctx context.Context, vp *domain.VehiclePosition) error) error {
	sub, err := s.js.Subscribe("transit.vehicle.>", func(msg *nats.Msg) {
		var vp domain.VehiclePosition
		if err := json.Unmarshal(msg.Data, &vp); err != nil {
			_ = msg.Nak()
			return
		}
		if err := handler(ctx, &vp); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	},
		nats.Durable("vehicle-processor"),
		nats.ManualAck(),
		nats.MaxDeliver(3),
	)
	if err != nil {
		return err
	}
	s.subs = append(s.subs, sub)
	return nil
}

func (s *Subscriber) SubscribeDelayEvents(ctx context.Context, handler func(ctx context.Context, event *domain.DelayEvent) error) error {
	sub, err := s.js.Subscribe("transit.delay.>", func(msg *nats.Msg) {
		var event domain.DelayEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			_ = msg.Nak()
			return
		}
		if err := handler(ctx, &event); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	},
		nats.Durable("delay-processor"),
		nats.ManualAck(),
		nats.MaxDeliver(3),
	)
	if err != nil {
		return err
	}
	s.subs = append(s.subs, sub)
	return nil
}

func (s *Subscriber) SubscribeDetourAlerts(ctx context.Context, handler func(ctx context.Context, tripID string) error) error {
	sub, err := s.js.Subscribe("transit.alerts.detour", func(msg *nats.Msg) {
		if err := handler(ctx, string(msg.Data)); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	},
		nats.Durable("detour-processor"),
		nats.ManualAck(),
		nats.MaxDeliver(3),
	)
	if err != nil {
		return err
	}
	s.subs = append(s.subs, sub)
	return nil
}

// Close unsubscribes and drains.
func (s *Subscriber) Close() {
	for _, sub := range s.subs {
		_ = sub.Unsubscribe()
	}
	_ = s.conn.Drain()
}
