package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/gtfsrt"
	"github.com/samirrijal/bilbopass/internal/pkg/config"
)

// ---------------------------------------------------------------------------
// Manifest types (same as ingestor)
// ---------------------------------------------------------------------------

type Manifest struct {
	Source   string        `json:"source"`
	Agencies []AgencyEntry `json:"agencies"`
}

type AgencyEntry struct {
	Name    string       `json:"name"`
	Slug    string       `json:"slug"`
	GTFSURL string       `json:"gtfs_url"`
	GTFSRT  *GTFSRTEntry `json:"gtfs_rt,omitempty"`
}

type GTFSRTEntry struct {
	VehiclePositions string `json:"vehicle_positions,omitempty"`
	TripUpdates      string `json:"trip_updates,omitempty"`
	Alerts           string `json:"alerts,omitempty"`
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	cfg, err := config.Load("bilbopass-realtime")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	pool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// NATS
	nc, err := nats.Connect(cfg.NATS.URL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		log.Fatalf("nats: %v", err)
	}
	defer nc.Drain()

	// Load manifest
	manifestPath := "manifest.json"
	if len(os.Args) > 1 {
		manifestPath = os.Args[1]
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		log.Fatalf("read manifest: %v", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		log.Fatalf("parse manifest: %v", err)
	}

	// Filter to agencies that have GTFS-RT feeds
	var rtAgencies []AgencyEntry
	for _, a := range manifest.Agencies {
		if a.GTFSRT != nil {
			rtAgencies = append(rtAgencies, a)
		}
	}

	log.Printf("BilboPass Realtime Poller — %d agencies with GTFS-RT feeds", len(rtAgencies))

	// Preload agency UUID map
	agencyIDs := make(map[string]string) // slug -> UUID
	for _, a := range rtAgencies {
		var id string
		err := pool.QueryRow(ctx, `SELECT id FROM agencies WHERE slug = $1`, a.Slug).Scan(&id)
		if err != nil {
			log.Printf("WARNING: agency %s not found in DB (run ingestor first): %v", a.Slug, err)
			continue
		}
		agencyIDs[a.Slug] = id
	}

	client := &http.Client{Timeout: 30 * time.Second}
	pollInterval := 30 * time.Second

	// Start polling loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	log.Printf("polling every %s", pollInterval)

	// Signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Run once immediately
	pollAll(ctx, pool, nc, client, rtAgencies, agencyIDs)

	for {
		select {
		case <-ticker.C:
			pollAll(ctx, pool, nc, client, rtAgencies, agencyIDs)
		case <-ctx.Done():
			return
		case sig := <-quit:
			log.Printf("received signal %v, shutting down realtime poller", sig)
			cancel()
			// Give in-flight polls time to finish
			time.Sleep(2 * time.Second)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Poll all agencies
// ---------------------------------------------------------------------------

func pollAll(ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn, client *http.Client, agencies []AgencyEntry, agencyIDs map[string]string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // max 8 concurrent fetches

	for _, a := range agencies {
		agencyID, ok := agencyIDs[a.Slug]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(agency AgencyEntry, aID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if agency.GTFSRT.VehiclePositions != "" {
				if err := pollVehiclePositions(ctx, pool, nc, client, agency, aID); err != nil {
					log.Printf("[%s] vehicle_positions: %v", agency.Slug, err)
				}
			}

			if agency.GTFSRT.TripUpdates != "" {
				if err := pollTripUpdates(ctx, pool, nc, client, agency, aID); err != nil {
					log.Printf("[%s] trip_updates: %v", agency.Slug, err)
				}
			}

			if agency.GTFSRT.Alerts != "" {
				if err := pollAlerts(ctx, nc, client, agency); err != nil {
					log.Printf("[%s] alerts: %v", agency.Slug, err)
				}
			}
		}(a, agencyID)
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Fetch + parse protobuf feed
// ---------------------------------------------------------------------------

func fetchFeed(client *http.Client, url string) (*gtfsrt.FeedMessage, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	feed := &gtfsrt.FeedMessage{}
	if err := proto.Unmarshal(body, feed); err != nil {
		return nil, fmt.Errorf("unmarshal protobuf: %w", err)
	}

	return feed, nil
}

// ---------------------------------------------------------------------------
// Vehicle Positions
// ---------------------------------------------------------------------------

func pollVehiclePositions(ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn, client *http.Client, agency AgencyEntry, agencyID string) error {
	feed, err := fetchFeed(client, agency.GTFSRT.VehiclePositions)
	if err != nil {
		return err
	}

	inserted := 0
	for _, entity := range feed.GetEntity() {
		vp := entity.GetVehicle()
		if vp == nil || vp.GetPosition() == nil {
			continue
		}

		pos := vp.GetPosition()
		trip := vp.GetTrip()
		vehicle := vp.GetVehicle()

		ts := time.Now()
		if vp.Timestamp != nil {
			ts = time.Unix(int64(vp.GetTimestamp()), 0)
		}

		vehicleID := ""
		if vehicle != nil {
			vehicleID = vehicle.GetId()
			if vehicleID == "" {
				vehicleID = vehicle.GetLabel()
			}
		}
		if vehicleID == "" {
			vehicleID = entity.GetId()
		}

		tripID := ""
		routeID := ""
		if trip != nil {
			tripID = trip.GetTripId()
			routeID = trip.GetRouteId()
		}

		// Insert into vehicle_positions (timescaledb hypertable)
		_, err := pool.Exec(ctx, `
			INSERT INTO vehicle_positions (time, vehicle_id, trip_id, route_id, location, bearing, speed, congestion_level, occupancy_status, metadata)
			VALUES ($1, $2,
				(SELECT id FROM trips WHERE trip_id = $3 AND route_id IN (SELECT id FROM routes WHERE agency_id = $9) LIMIT 1),
				(SELECT id FROM routes WHERE route_id = $4 AND agency_id = $9 LIMIT 1),
				ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography,
				$7, $8, $10, $11, $12)
		`, ts, vehicleID,
			nilEmpty(tripID), nilEmpty(routeID),
			float64(pos.GetLongitude()), float64(pos.GetLatitude()),
			float64(pos.GetBearing()), float64(pos.GetSpeed()),
			agencyID,
			int(vp.GetCongestionLevel()), int(vp.GetOccupancyStatus()),
			map[string]any{"agency": agency.Slug},
		)
		if err != nil {
			// Log but continue — some trip/route IDs may not resolve
			if !strings.Contains(err.Error(), "null value in column") {
				log.Printf("[%s] insert vp %s: %v", agency.Slug, vehicleID, err)
			}
			continue
		}
		inserted++

		// Publish to NATS for WebSocket clients
		vpDomain := domain.VehiclePosition{
			Time:      ts,
			VehicleID: vehicleID,
			TripID:    tripID,
			RouteID:   routeID,
			Location: domain.GeoPoint{
				Lat: float64(pos.GetLatitude()),
				Lon: float64(pos.GetLongitude()),
			},
			Bearing:         float64(pos.GetBearing()),
			Speed:           float64(pos.GetSpeed()),
			CongestionLevel: int(vp.GetCongestionLevel()),
			OccupancyStatus: int(vp.GetOccupancyStatus()),
			Metadata:        map[string]any{"agency": agency.Slug},
		}
		if data, err := json.Marshal(vpDomain); err == nil {
			_ = nc.Publish(fmt.Sprintf("transit.vehicle.%s.%s", agency.Slug, vehicleID), data)
		}
	}

	if inserted > 0 {
		log.Printf("[%s] %d vehicle positions", agency.Slug, inserted)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Trip Updates (delay detection)
// ---------------------------------------------------------------------------

func pollTripUpdates(ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn, client *http.Client, agency AgencyEntry, agencyID string) error {
	feed, err := fetchFeed(client, agency.GTFSRT.TripUpdates)
	if err != nil {
		return err
	}

	delays := 0
	for _, entity := range feed.GetEntity() {
		tu := entity.GetTripUpdate()
		if tu == nil {
			continue
		}

		trip := tu.GetTrip()
		tripID := trip.GetTripId()

		// Check overall delay
		overallDelay := int(tu.GetDelay())

		// Also check per-stop delays
		for _, stu := range tu.GetStopTimeUpdate() {
			stopDelay := 0
			if arr := stu.GetArrival(); arr != nil && arr.Delay != nil {
				stopDelay = int(arr.GetDelay())
			} else if dep := stu.GetDeparture(); dep != nil && dep.Delay != nil {
				stopDelay = int(dep.GetDelay())
			} else {
				stopDelay = overallDelay
			}

			// If delay > 3 minutes, publish alert
			if stopDelay > 180 {
				delays++
				alertData, _ := json.Marshal(map[string]any{
					"agency":    agency.Slug,
					"trip_id":   tripID,
					"stop_id":   stu.GetStopId(),
					"delay_sec": stopDelay,
					"route_id":  trip.GetRouteId(),
				})
				_ = nc.Publish("transit.delays.detected", alertData)
			}
		}
	}

	if delays > 0 {
		log.Printf("[%s] %d significant delays detected", agency.Slug, delays)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Alerts
// ---------------------------------------------------------------------------

func pollAlerts(ctx context.Context, nc *nats.Conn, client *http.Client, agency AgencyEntry) error {
	feed, err := fetchFeed(client, agency.GTFSRT.Alerts)
	if err != nil {
		return err
	}

	for _, entity := range feed.GetEntity() {
		alert := entity.GetAlert()
		if alert == nil {
			continue
		}

		headerText := ""
		if ht := alert.GetHeaderText(); ht != nil {
			for _, t := range ht.GetTranslation() {
				headerText = t.GetText()
				break
			}
		}

		descText := ""
		if dt := alert.GetDescriptionText(); dt != nil {
			for _, t := range dt.GetTranslation() {
				descText = t.GetText()
				break
			}
		}

		if headerText == "" && descText == "" {
			continue
		}

		// Get affected routes/stops
		var routeIDs, stopIDs []string
		for _, ie := range alert.GetInformedEntity() {
			if r := ie.GetRouteId(); r != "" {
				routeIDs = append(routeIDs, r)
			}
			if s := ie.GetStopId(); s != "" {
				stopIDs = append(stopIDs, s)
			}
		}

		alertData, _ := json.Marshal(map[string]any{
			"agency":      agency.Slug,
			"header":      headerText,
			"description": descText,
			"cause":       alert.GetCause().String(),
			"effect":      alert.GetEffect().String(),
			"route_ids":   routeIDs,
			"stop_ids":    stopIDs,
		})
		_ = nc.Publish(fmt.Sprintf("transit.alerts.%s", agency.Slug), alertData)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nilEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
