package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// Publisher implements ports.EventPublisher using NATS JetStream.
type Publisher struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// NewPublisher connects to NATS and enables JetStream.
func NewPublisher(url string) (*Publisher, error) {
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

	// Ensure streams exist
	streams := []nats.StreamConfig{
		{
			Name:      "VEHICLE_POSITIONS",
			Subjects:  []string{"transit.vehicle.>"},
			Retention: nats.WorkQueuePolicy,
			MaxAge:    1 * time.Hour,
			Storage:   nats.FileStorage,
		},
		{
			Name:      "TRANSIT_ALERTS",
			Subjects:  []string{"transit.alerts.>"},
			Retention: nats.InterestPolicy,
			MaxAge:    24 * time.Hour,
			Storage:   nats.FileStorage,
		},
		{
			Name:      "TRANSIT_DELAYS",
			Subjects:  []string{"transit.delay.>"},
			Retention: nats.WorkQueuePolicy,
			MaxAge:    24 * time.Hour,
			Storage:   nats.FileStorage,
		},
	}

	for _, cfg := range streams {
		if _, err := js.AddStream(&cfg); err != nil {
			// Stream may already exist — try update
			if _, err := js.UpdateStream(&cfg); err != nil {
				return nil, fmt.Errorf("ensure stream %s: %w", cfg.Name, err)
			}
		}
	}

	return &Publisher{conn: conn, js: js}, nil
}

func (p *Publisher) PublishVehiclePosition(ctx context.Context, vp *domain.VehiclePosition) error {
	data, err := json.Marshal(vp)
	if err != nil {
		return err
	}
	_, err = p.js.Publish("transit.vehicle."+vp.VehicleID, data)
	return err
}

func (p *Publisher) PublishDelayEvent(ctx context.Context, event *domain.DelayEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = p.js.Publish("transit.delay."+event.TripID, data)
	return err
}

func (p *Publisher) PublishDetourAlert(ctx context.Context, tripID string) error {
	_, err := p.js.Publish("transit.alerts.detour", []byte(tripID))
	return err
}

func (p *Publisher) PublishBroadcast(ctx context.Context, data []byte) error {
	return p.conn.Publish("transit.updates.broadcast", data)
}

// Close drains and closes the connection.
func (p *Publisher) Close() {
	_ = p.conn.Drain()
}

// RawConn creates a plain NATS connection for subscribing (e.g. WebSocket relay).
func RawConn(url string) (*nats.Conn, error) {
	return nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
}
