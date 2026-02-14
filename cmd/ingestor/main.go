package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samirrijal/bilbopass/internal/pkg/config"
)

// ---------------------------------------------------------------------------
// Manifest types
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
	cfg, err := config.Load("bilbopass-ingestor")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

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

	log.Printf("BilboPass GTFS Ingestor — %d agencies from %s", len(manifest.Agencies), manifest.Source)

	// Filter agencies (optional CLI arg: slug list)
	slugFilter := map[string]bool{}
	if len(os.Args) > 2 {
		for _, s := range strings.Split(os.Args[2], ",") {
			slugFilter[strings.TrimSpace(s)] = true
		}
	}

	client := &http.Client{Timeout: 120 * time.Second}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // max 4 concurrent downloads

	for _, agency := range manifest.Agencies {
		if len(slugFilter) > 0 && !slugFilter[agency.Slug] {
			continue
		}

		wg.Add(1)
		go func(a AgencyEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := ingestAgency(ctx, pool, client, a); err != nil {
				log.Printf("ERROR [%s]: %v", a.Slug, err)
			}
		}(agency)
	}

	wg.Wait()
	log.Println("ingestion complete")
}

// ---------------------------------------------------------------------------
// Per-agency ingestion
// ---------------------------------------------------------------------------

func ingestAgency(ctx context.Context, pool *pgxpool.Pool, client *http.Client, agency AgencyEntry) error {
	log.Printf("[%s] downloading GTFS from %s", agency.Slug, agency.GTFSURL)

	resp, err := client.Get(agency.GTFSURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, agency.GTFSURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	// Upsert agency
	agencyID, err := upsertAgency(ctx, pool, agency)
	if err != nil {
		return fmt.Errorf("upsert agency: %w", err)
	}
	log.Printf("[%s] agency_id=%s", agency.Slug, agencyID)

	// Process GTFS files in order (stops before stop_times, routes before trips)
	if err := processStops(ctx, pool, zr, agencyID, agency.Slug); err != nil {
		log.Printf("[%s] stops: %v", agency.Slug, err)
	}
	if err := processRoutes(ctx, pool, zr, agencyID, agency.Slug); err != nil {
		log.Printf("[%s] routes: %v", agency.Slug, err)
	}
	if err := processTrips(ctx, pool, zr, agencyID, agency.Slug); err != nil {
		log.Printf("[%s] trips: %v", agency.Slug, err)
	}
	if err := processStopTimes(ctx, pool, zr, agencyID, agency.Slug); err != nil {
		log.Printf("[%s] stop_times: %v", agency.Slug, err)
	}
	if err := processShapes(ctx, pool, zr, agencyID, agency.Slug); err != nil {
		log.Printf("[%s] shapes: %v (may not exist)", agency.Slug, err)
	}

	log.Printf("[%s] done", agency.Slug)
	return nil
}

// ---------------------------------------------------------------------------
// Agency upsert
// ---------------------------------------------------------------------------

func upsertAgency(ctx context.Context, pool *pgxpool.Pool, a AgencyEntry) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO agencies (slug, name, url, timezone)
		VALUES ($1, $2, $3, 'Europe/Madrid')
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, url = EXCLUDED.url
		RETURNING id
	`, a.Slug, a.Name, a.GTFSURL).Scan(&id)
	return id, err
}

// ---------------------------------------------------------------------------
// Stops
// ---------------------------------------------------------------------------

func processStops(ctx context.Context, pool *pgxpool.Pool, zr *zip.Reader, agencyID, slug string) error {
	f, err := openCSV(zr, "stops.txt")
	if err != nil {
		return err
	}

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	header, err := reader.Read()
	if err != nil {
		return err
	}
	cols := indexColumns(header)

	const batchSize = 500
	batch := &pgx.Batch{}
	count := 0
	total := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		stopID := strings.TrimSpace(record[cols["stop_id"]])
		name := strings.TrimSpace(record[cols["stop_name"]])
		lat, _ := strconv.ParseFloat(strings.TrimSpace(record[cols["stop_lat"]]), 64)
		lon, _ := strconv.ParseFloat(strings.TrimSpace(record[cols["stop_lon"]]), 64)
		platformCode := getField(record, cols, "platform_code")
		wheelchair := getField(record, cols, "wheelchair_boarding") == "1"

		if lat == 0 && lon == 0 {
			continue
		}

		batch.Queue(`
			INSERT INTO stops (stop_id, agency_id, name, location, platform_code, wheelchair_accessible)
			VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, $7)
			ON CONFLICT (agency_id, stop_id) DO UPDATE
			SET name = EXCLUDED.name, location = EXCLUDED.location,
			    platform_code = EXCLUDED.platform_code,
			    wheelchair_accessible = EXCLUDED.wheelchair_accessible
		`, stopID, agencyID, name, lon, lat, nilEmpty(platformCode), wheelchair)

		count++
		total++

		if count >= batchSize {
			if err := flushBatch(ctx, pool, batch, count); err != nil {
				return err
			}
			batch = &pgx.Batch{}
			count = 0
		}
	}

	if count > 0 {
		if err := flushBatch(ctx, pool, batch, count); err != nil {
			return err
		}
	}

	log.Printf("[%s]   stops: %d", slug, total)
	return nil
}

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

func processRoutes(ctx context.Context, pool *pgxpool.Pool, zr *zip.Reader, agencyID, slug string) error {
	f, err := openCSV(zr, "routes.txt")
	if err != nil {
		return err
	}

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	header, err := reader.Read()
	if err != nil {
		return err
	}
	cols := indexColumns(header)

	batch := &pgx.Batch{}
	count := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		routeID := record[cols["route_id"]]
		shortName := getField(record, cols, "route_short_name")
		longName := getField(record, cols, "route_long_name")
		routeType, _ := strconv.Atoi(getField(record, cols, "route_type"))
		color := getField(record, cols, "route_color")
		textColor := getField(record, cols, "route_text_color")

		if longName == "" {
			longName = shortName
		}
		if longName == "" {
			longName = routeID
		}
		if color == "" {
			color = "000000"
		}
		if textColor == "" {
			textColor = "FFFFFF"
		}

		batch.Queue(`
			INSERT INTO routes (route_id, agency_id, short_name, long_name, route_type, color, text_color)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (agency_id, route_id) DO UPDATE
			SET short_name = EXCLUDED.short_name, long_name = EXCLUDED.long_name,
			    route_type = EXCLUDED.route_type, color = EXCLUDED.color, text_color = EXCLUDED.text_color
		`, routeID, agencyID, shortName, longName, routeType, color, textColor)

		count++
	}

	if count > 0 {
		if err := flushBatch(ctx, pool, batch, count); err != nil {
			return err
		}
	}

	log.Printf("[%s]   routes: %d", slug, count)
	return nil
}

// ---------------------------------------------------------------------------
// Trips
// ---------------------------------------------------------------------------

func processTrips(ctx context.Context, pool *pgxpool.Pool, zr *zip.Reader, agencyID, slug string) error {
	f, err := openCSV(zr, "trips.txt")
	if err != nil {
		return err
	}

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	header, err := reader.Read()
	if err != nil {
		return err
	}
	cols := indexColumns(header)

	const batchSize = 500
	batch := &pgx.Batch{}
	count := 0
	total := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		tripID := record[cols["trip_id"]]
		routeID := record[cols["route_id"]]
		serviceID := record[cols["service_id"]]
		headsign := getField(record, cols, "trip_headsign")
		directionID, _ := strconv.Atoi(getField(record, cols, "direction_id"))
		shapeID := getField(record, cols, "shape_id")
		wheelchair := getField(record, cols, "wheelchair_accessible") == "1"
		bikes := getField(record, cols, "bikes_allowed") == "1"

		// We need the route's internal UUID. Use a subquery.
		batch.Queue(`
			INSERT INTO trips (trip_id, route_id, service_id, headsign, direction_id, shape_id, wheelchair_accessible, bikes_allowed)
			VALUES ($1, (SELECT id FROM routes WHERE route_id = $2 AND agency_id = $3), $4, $5, $6, $7, $8, $9)
			ON CONFLICT (route_id, trip_id) DO UPDATE
			SET service_id = EXCLUDED.service_id, headsign = EXCLUDED.headsign,
			    direction_id = EXCLUDED.direction_id, shape_id = EXCLUDED.shape_id
		`, tripID, routeID, agencyID, serviceID, headsign, directionID, shapeID, wheelchair, bikes)

		count++
		total++

		if count >= batchSize {
			if err := flushBatch(ctx, pool, batch, count); err != nil {
				return err
			}
			batch = &pgx.Batch{}
			count = 0
		}
	}

	if count > 0 {
		if err := flushBatch(ctx, pool, batch, count); err != nil {
			return err
		}
	}

	log.Printf("[%s]   trips: %d", slug, total)
	return nil
}

// ---------------------------------------------------------------------------
// Stop Times
// ---------------------------------------------------------------------------

func processStopTimes(ctx context.Context, pool *pgxpool.Pool, zr *zip.Reader, agencyID, slug string) error {
	f, err := openCSV(zr, "stop_times.txt")
	if err != nil {
		return err
	}

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	header, err := reader.Read()
	if err != nil {
		return err
	}
	cols := indexColumns(header)

	const batchSize = 1000
	batch := &pgx.Batch{}
	count := 0
	total := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		tripID := record[cols["trip_id"]]
		stopID := record[cols["stop_id"]]
		arrivalStr := record[cols["arrival_time"]]
		departureStr := record[cols["departure_time"]]
		stopSeq, _ := strconv.Atoi(record[cols["stop_sequence"]])
		pickupType, _ := strconv.Atoi(getField(record, cols, "pickup_type"))
		dropOffType, _ := strconv.Atoi(getField(record, cols, "drop_off_type"))

		arrival := parseGTFSTime(arrivalStr)
		departure := parseGTFSTime(departureStr)

		// Use subqueries to resolve trip_id and stop_id to internal UUIDs.
		// This is intentionally ON CONFLICT DO NOTHING to skip duplicates from re-runs.
		batch.Queue(`
			INSERT INTO stop_times (trip_id, stop_id, arrival_time, departure_time, stop_sequence, pickup_type, drop_off_type)
			VALUES (
				(SELECT id FROM trips WHERE trip_id = $1 AND route_id IN (SELECT id FROM routes WHERE agency_id = $6)),
				(SELECT id FROM stops WHERE stop_id = $2 AND agency_id = $6),
				$3, $4, $5, $7, $8
			)
			ON CONFLICT DO NOTHING
		`, tripID, stopID, arrival, departure, stopSeq, agencyID, pickupType, dropOffType)

		count++
		total++

		if count >= batchSize {
			if err := flushBatch(ctx, pool, batch, count); err != nil {
				log.Printf("[%s]   stop_times batch error (continuing): %v", slug, err)
			}
			batch = &pgx.Batch{}
			count = 0
		}
	}

	if count > 0 {
		if err := flushBatch(ctx, pool, batch, count); err != nil {
			log.Printf("[%s]   stop_times final batch error: %v", slug, err)
		}
	}

	log.Printf("[%s]   stop_times: %d", slug, total)
	return nil
}

// ---------------------------------------------------------------------------
// Shapes → route geometry
// ---------------------------------------------------------------------------

func processShapes(ctx context.Context, pool *pgxpool.Pool, zr *zip.Reader, agencyID, slug string) error {
	f, err := openCSV(zr, "shapes.txt")
	if err != nil {
		return err // shapes.txt is optional
	}

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	header, err := reader.Read()
	if err != nil {
		return err
	}
	cols := indexColumns(header)

	// Collect shape points grouped by shape_id
	type shapePoint struct {
		Lat float64
		Lon float64
		Seq int
	}
	shapes := make(map[string][]shapePoint)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		shapeID := record[cols["shape_id"]]
		lat, _ := strconv.ParseFloat(record[cols["shape_pt_lat"]], 64)
		lon, _ := strconv.ParseFloat(record[cols["shape_pt_lon"]], 64)
		seq, _ := strconv.Atoi(record[cols["shape_pt_sequence"]])

		shapes[shapeID] = append(shapes[shapeID], shapePoint{lat, lon, seq})
	}

	// Sort each shape's points by sequence and build WKT LINESTRING
	updated := 0
	for shapeID, pts := range shapes {
		if len(pts) < 2 {
			continue
		}

		// Sort by sequence
		sort.Slice(pts, func(i, j int) bool {
			return pts[i].Seq < pts[j].Seq
		})

		// Build LINESTRING WKT
		var sb strings.Builder
		sb.WriteString("LINESTRING(")
		for i, p := range pts {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, "%f %f", p.Lon, p.Lat)
		}
		sb.WriteString(")")

		// Update routes that reference this shape_id via trips
		_, err := pool.Exec(ctx, `
			UPDATE routes SET shape = ST_GeogFromText($1)
			WHERE id IN (
				SELECT DISTINCT r.id FROM routes r
				JOIN trips t ON t.route_id = r.id
				WHERE t.shape_id = $2 AND r.agency_id = $3
			) AND shape IS NULL
		`, sb.String(), shapeID, agencyID)
		if err != nil {
			log.Printf("[%s]   shape %s error: %v", slug, shapeID, err)
			continue
		}
		updated++
	}

	log.Printf("[%s]   shapes: %d unique, %d applied to routes", slug, len(shapes), updated)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func openCSV(zr *zip.Reader, name string) (io.ReadCloser, error) {
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("file %s not found in zip", name)
}

func indexColumns(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, col := range header {
		// Strip BOM from first column
		col = strings.TrimPrefix(col, "\xef\xbb\xbf")
		m[strings.TrimSpace(col)] = i
	}
	return m
}

func getField(record []string, cols map[string]int, name string) string {
	idx, ok := cols[name]
	if !ok || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

func nilEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func flushBatch(ctx context.Context, pool *pgxpool.Pool, batch *pgx.Batch, count int) error {
	br := pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < count; i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch item %d: %w", i, err)
		}
	}
	return nil
}

// parseGTFSTime parses "HH:MM:SS" into a time.Duration (allows HH > 23).
func parseGTFSTime(s string) time.Duration {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec, _ := strconv.Atoi(parts[2])
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}
