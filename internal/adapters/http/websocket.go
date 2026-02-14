package http

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/nats-io/nats.go"
)

// wsMessage is sent from client to subscribe/unsubscribe to feeds.
type wsMessage struct {
	Action  string `json:"action"`  // "subscribe" | "unsubscribe"
	Agency  string `json:"agency"`  // agency slug filter (optional, "" = all)
	Channel string `json:"channel"` // "vehicles" | "alerts" | "delays" (default: vehicles)
}

// WebSocketHandler returns a handler that upgrades to WebSocket
// and relays real-time NATS events to connected clients.
// Clients send JSON: {"action":"subscribe","agency":"metro_bilbao","channel":"vehicles"}
// An empty agency means all agencies. Default channel is "vehicles".
func WebSocketHandler(nc *nats.Conn) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		defer c.Close()

		remoteAddr := c.RemoteAddr().String()
		log.Printf("ws client connected: %s", remoteAddr)

		var mu sync.Mutex
		subs := make(map[string]*nats.Subscription) // subject -> subscription

		// Helper: thread-safe write
		writeJSON := func(v interface{}) error {
			data, err := json.Marshal(v)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			return c.WriteMessage(websocket.TextMessage, data)
		}

		// Auto-subscribe to all vehicle positions by default
		defaultSubject := "transit.vehicle.>"
		sub, err := nc.Subscribe(defaultSubject, func(msg *nats.Msg) {
			_ = writeJSON(json.RawMessage(msg.Data))
		})
		if err != nil {
			log.Printf("ws default subscribe error: %v", err)
			return
		}
		subs[defaultSubject] = sub

		// Keep-alive ping
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					mu.Lock()
					err := c.WriteMessage(websocket.PingMessage, nil)
					mu.Unlock()
					if err != nil {
						return
					}
				case <-done:
					return
				}
			}
		}()

		// Read client messages for subscribe/unsubscribe
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				break
			}

			var m wsMessage
			if err := json.Unmarshal(msg, &m); err != nil {
				_ = writeJSON(map[string]string{"error": "invalid JSON"})
				continue
			}

			// Build NATS subject
			channel := m.Channel
			if channel == "" {
				channel = "vehicles"
			}

			var subject string
			switch channel {
			case "vehicles":
				if m.Agency != "" {
					subject = "transit.vehicle." + m.Agency + ".>"
				} else {
					subject = "transit.vehicle.>"
				}
			case "alerts":
				if m.Agency != "" {
					subject = "transit.alerts." + m.Agency
				} else {
					subject = "transit.alerts.>"
				}
			case "delays":
				subject = "transit.delays.detected"
			default:
				_ = writeJSON(map[string]string{"error": "unknown channel: " + channel})
				continue
			}

			switch m.Action {
			case "subscribe":
				if _, exists := subs[subject]; exists {
					_ = writeJSON(map[string]string{"status": "already subscribed", "subject": subject})
					continue
				}
				s, err := nc.Subscribe(subject, func(msg *nats.Msg) {
					_ = writeJSON(json.RawMessage(msg.Data))
				})
				if err != nil {
					_ = writeJSON(map[string]string{"error": "subscribe failed: " + err.Error()})
					continue
				}
				subs[subject] = s
				_ = writeJSON(map[string]string{"status": "subscribed", "subject": subject})

			case "unsubscribe":
				if s, exists := subs[subject]; exists {
					_ = s.Unsubscribe()
					delete(subs, subject)
					_ = writeJSON(map[string]string{"status": "unsubscribed", "subject": subject})
				} else {
					_ = writeJSON(map[string]string{"error": "not subscribed to " + subject})
				}

			default:
				_ = writeJSON(map[string]string{"error": "unknown action: " + m.Action})
			}
		}

		// Cleanup
		close(done)
		for _, s := range subs {
			_ = s.Unsubscribe()
		}
		log.Printf("ws client disconnected: %s", remoteAddr)
	}
}
