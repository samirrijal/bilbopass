package http

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// FeedStats holds statistics about the ingested GTFS data.
type FeedStats struct {
	Agencies   int    `json:"agencies"`
	Stops      int    `json:"stops"`
	Routes     int    `json:"routes"`
	Trips      int    `json:"trips"`
	StopTimes  int    `json:"stop_times"`
	LastIngest string `json:"last_ingest,omitempty"`
}

// FeedStatsHandler returns row counts from the transit tables.
func FeedStatsHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if deps.DB == nil {
			return errInternal(c, "database not available")
		}

		var stats FeedStats
		row := deps.DB.Pool.QueryRow(c.Context(), `
			SELECT
				(SELECT count(*) FROM agencies),
				(SELECT count(*) FROM stops),
				(SELECT count(*) FROM routes),
				(SELECT count(*) FROM trips),
				(SELECT count(*) FROM stop_times),
				COALESCE((SELECT max(created_at)::text FROM stops), '')
		`)
		if err := row.Scan(&stats.Agencies, &stats.Stops, &stats.Routes,
			&stats.Trips, &stats.StopTimes, &stats.LastIngest); err != nil {
			return errInternal(c, err.Error())
		}

		c.Set("Cache-Control", "public, max-age=60")
		return c.JSON(stats)
	}
}

// ListAgenciesHandler returns all transit agencies.
func ListAgenciesHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		agencies, err := deps.Agencies.List(c.Context())
		if err != nil {
			return errInternal(c, err.Error())
		}

		// Apply offset/limit pagination on the full list
		offset := c.QueryInt("offset", 0)
		limit := c.QueryInt("limit", 100)
		if offset < 0 {
			offset = 0
		}
		if limit <= 0 || limit > 200 {
			limit = 100
		}

		total := len(agencies)
		if offset >= total {
			agencies = nil
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			agencies = agencies[offset:end]
		}

		pg := Pagination{Offset: offset, Limit: limit, Total: total}
		SetLinkHeaders(c, pg)
		return c.JSON(PaginatedResponse{Data: agencies, Pagination: pg})
	}
}

// NearbyStopsHandler returns stops within a radius of a point.
func NearbyStopsHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		lat := c.QueryFloat("lat", 0)
		lon := c.QueryFloat("lon", 0)
		radius := c.QueryFloat("radius", 500)
		limit := c.QueryInt("limit", 50)

		if lat == 0 || lon == 0 {
			return errBadRequest(c, "lat and lon are required")
		}
		if radius <= 0 || radius > 10000 {
			return errBadRequest(c, "radius must be between 1 and 10000 meters")
		}
		if limit <= 0 || limit > 200 {
			limit = 50
		}

		stops, err := deps.Stops.FindNearby(c.Context(), lat, lon, radius, limit)
		if err != nil {
			return errInternal(c, err.Error())
		}

		c.Set("Cache-Control", "public, max-age=300")
		return c.JSON(stops)
	}
}

// SearchStopsHandler performs fuzzy search on stop names.
func SearchStopsHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		query := c.Query("q")
		if query == "" {
			return errBadRequest(c, "q query parameter is required")
		}
		if len(query) > 200 {
			return errBadRequest(c, "query too long (max 200 characters)")
		}
		limit := c.QueryInt("limit", 20)
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		stops, err := deps.Stops.Search(c.Context(), query, nil, limit)
		if err != nil {
			return errInternal(c, err.Error())
		}

		return c.JSON(stops)
	}
}

// GetStopHandler returns a single stop by ID.
func GetStopHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "stop id is required")
		}
		stop, err := deps.Stops.GetByID(c.Context(), id)
		if err != nil {
			return errNotFound(c, "stop not found")
		}
		return c.JSON(stop)
	}
}

// GetRouteHandler returns a route by ID.
func GetRouteHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "route id is required")
		}
		route, err := deps.Routes.GetByID(c.Context(), id)
		if err != nil {
			return errNotFound(c, "route not found")
		}
		return c.JSON(route)
	}
}

// GetRouteVehiclesHandler returns live vehicle positions for a route.
func GetRouteVehiclesHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "route id is required")
		}
		vehicles, err := deps.Routes.GetLiveVehicles(c.Context(), id)
		if err != nil {
			return errInternal(c, err.Error())
		}
		return c.JSON(vehicles)
	}
}

// ListRoutesHandler lists routes, optionally filtered by agency.
func ListRoutesHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		agencyID := c.Query("agency_id")
		if agencyID == "" {
			return errBadRequest(c, "agency_id query parameter is required")
		}

		routes, err := deps.Routes.ListByAgency(c.Context(), agencyID)
		if err != nil {
			return errInternal(c, err.Error())
		}

		// Apply offset/limit pagination
		offset := c.QueryInt("offset", 0)
		limit := c.QueryInt("limit", 100)
		if offset < 0 {
			offset = 0
		}
		if limit <= 0 || limit > 500 {
			limit = 100
		}

		total := len(routes)
		if offset >= total {
			routes = nil
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			routes = routes[offset:end]
		}

		pg := Pagination{Offset: offset, Limit: limit, Total: total}
		SetLinkHeaders(c, pg)
		return c.JSON(PaginatedResponse{Data: routes, Pagination: pg})
	}
}

// StopDeparturesHandler returns next scheduled departures at a stop.
func StopDeparturesHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "stop id is required")
		}
		limit := c.QueryInt("limit", 10)
		if limit <= 0 || limit > 50 {
			limit = 10
		}

		departures, err := deps.Departures.NextDeparturesAtStop(c.Context(), id, limit)
		if err != nil {
			return errInternal(c, err.Error())
		}
		return c.JSON(departures)
	}
}

// GetAgencyHandler returns a single agency by slug.
func GetAgencyHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		slug := c.Params("slug")
		if slug == "" {
			return errBadRequest(c, "agency slug is required")
		}
		agency, err := deps.Agencies.GetBySlug(c.Context(), slug)
		if err != nil {
			return errNotFound(c, "agency not found")
		}
		return c.JSON(agency)
	}
}

// AgencyRoutesHandler returns all routes belonging to an agency (by slug).
func AgencyRoutesHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		slug := c.Params("slug")
		if slug == "" {
			return errBadRequest(c, "agency slug is required")
		}

		// Resolve slug → agency ID
		agency, err := deps.Agencies.GetBySlug(c.Context(), slug)
		if err != nil {
			return errNotFound(c, "agency not found")
		}

		routes, err := deps.Routes.ListByAgency(c.Context(), agency.ID)
		if err != nil {
			return errInternal(c, err.Error())
		}

		// Pagination
		offset := c.QueryInt("offset", 0)
		limit := c.QueryInt("limit", 100)
		if offset < 0 {
			offset = 0
		}
		if limit <= 0 || limit > 500 {
			limit = 100
		}

		total := len(routes)
		if offset >= total {
			routes = nil
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			routes = routes[offset:end]
		}

		pg := Pagination{Offset: offset, Limit: limit, Total: total}
		SetLinkHeaders(c, pg)
		return c.JSON(PaginatedResponse{Data: routes, Pagination: pg})
	}
}

// StopRoutesHandler returns the routes that serve a given stop.
func StopRoutesHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "stop id is required")
		}
		routes, err := deps.Routes.ListByStop(c.Context(), id)
		if err != nil {
			return errInternal(c, err.Error())
		}
		return c.JSON(routes)
	}
}

// BatchStopsHandler returns multiple stops by ID.
func BatchStopsHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ids := c.Query("ids", "")
		if ids == "" {
			return errBadRequest(c, "ids query parameter is required (comma-separated)")
		}

		// Parse comma-separated IDs
		var stopIDs []string
		for _, id := range strings.Split(ids, ",") {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				stopIDs = append(stopIDs, trimmed)
			}
		}

		if len(stopIDs) == 0 {
			return errBadRequest(c, "at least one stop ID is required")
		}
		if len(stopIDs) > 100 {
			return errBadRequest(c, "maximum 100 stop IDs allowed")
		}

		stops, err := deps.Stops.GetByIDs(c.Context(), stopIDs)
		if err != nil {
			return errInternal(c, err.Error())
		}

		// ETag for batch too
		c.Set("Cache-Control", "public, max-age=300")
		return c.JSON(stops)
	}
}

// GetTripHandler returns a single trip by ID.
func GetTripHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "trip id is required")
		}
		trip, err := deps.Trips.GetByID(c.Context(), id)
		if err != nil {
			return errNotFound(c, "trip not found")
		}
		return c.JSON(trip)
	}
}

// TripStopTimesHandler returns the stop-times for a trip, ordered by sequence.
func TripStopTimesHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "trip id is required")
		}
		stopTimes, err := deps.Trips.GetStopTimes(c.Context(), id)
		if err != nil {
			return errInternal(c, err.Error())
		}
		return c.JSON(stopTimes)
	}
}

// JourneyHandler plans a journey between two stops.
// GET /v1/journeys?from=<stop_uuid>&to=<stop_uuid>&depart_at=15:30&max_transfers=1
// GET /v1/journeys?from_name=Abando&to_name=Sarriko
func JourneyHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fromID := c.Query("from")
		toID := c.Query("to")
		fromName := c.Query("from_name")
		toName := c.Query("to_name")

		// Parse optional departure time (HH:MM or full ISO)
		var departAt *time.Time
		if raw := c.Query("depart_at"); raw != "" {
			// Try HH:MM format first
			if t, err := time.Parse("15:04", raw); err == nil {
				now := time.Now()
				full := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
				departAt = &full
			} else if t, err := time.Parse(time.RFC3339, raw); err == nil {
				departAt = &t
			}
		}

		maxTransfers := c.QueryInt("max_transfers", 1)

		// By name or by ID
		if fromName != "" && toName != "" {
			journeys, err := deps.Journeys.PlanJourneyByName(c.Context(), fromName, toName, departAt)
			if err != nil {
				return errBadRequest(c, err.Error())
			}
			return c.JSON(journeyResponse(journeys))
		}

		if fromID == "" || toID == "" {
			return errBadRequest(c, "from and to (stop UUIDs) or from_name and to_name are required")
		}

		journeys, err := deps.Journeys.PlanJourney(c.Context(), fromID, toID, departAt, maxTransfers)
		if err != nil {
			return errBadRequest(c, err.Error())
		}

		return c.JSON(journeyResponse(journeys))
	}
}

// journeyResponse formats journeys with human-readable durations.
func journeyResponse(journeys []domain.Journey) fiber.Map {
	type legResp struct {
		Route       interface{} `json:"route"`
		FromStop    interface{} `json:"from_stop"`
		ToStop      interface{} `json:"to_stop"`
		DepartureAt string      `json:"departure_at"`
		ArrivalAt   string      `json:"arrival_at"`
	}

	type journeyResp struct {
		Legs          []legResp `json:"legs"`
		DepartureTime string    `json:"departure_time"`
		ArrivalTime   string    `json:"arrival_time"`
		Duration      string    `json:"duration"`
		DurationMin   int       `json:"duration_minutes"`
		Transfers     int       `json:"transfers"`
	}

	var results []journeyResp
	for _, j := range journeys {
		var legs []legResp
		for _, l := range j.Legs {
			legs = append(legs, legResp{
				Route: fiber.Map{
					"id":         l.Route.ID,
					"short_name": l.Route.ShortName,
					"long_name":  l.Route.LongName,
					"color":      l.Route.Color,
					"route_type": l.Route.RouteType,
				},
				FromStop: fiber.Map{
					"id":       l.FromStop.ID,
					"name":     l.FromStop.Name,
					"location": l.FromStop.Location,
				},
				ToStop: fiber.Map{
					"id":       l.ToStop.ID,
					"name":     l.ToStop.Name,
					"location": l.ToStop.Location,
				},
				DepartureAt: l.Departure.ScheduledTime.Format("15:04"),
				ArrivalAt:   l.ArrivalTime.Format("15:04"),
			})
		}
		results = append(results, journeyResp{
			Legs:          legs,
			DepartureTime: j.DepartureTime.Format("15:04"),
			ArrivalTime:   j.ArrivalTime.Format("15:04"),
			Duration:      j.Duration.String(),
			DurationMin:   int(j.Duration.Minutes()),
			Transfers:     j.Transfers,
		})
	}

	return fiber.Map{
		"journeys": results,
		"count":    len(results),
	}
}

// AgencyStatsHandler returns detailed stats for a single agency.
func AgencyStatsHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		slug := c.Params("slug")
		if slug == "" {
			return errBadRequest(c, "agency slug is required")
		}

		agency, err := deps.Agencies.GetBySlug(c.Context(), slug)
		if err != nil {
			return errNotFound(c, "agency not found")
		}

		if deps.DB == nil {
			return errInternal(c, "database not available")
		}

		var stats struct {
			Stops     int    `json:"stops"`
			Routes    int    `json:"routes"`
			Trips     int    `json:"trips"`
			StopTimes int    `json:"stop_times"`
			LastSync  string `json:"last_sync"`
		}

		row := deps.DB.Pool.QueryRow(c.Context(), `
            SELECT
                (SELECT count(*) FROM stops WHERE agency_id = $1),
                (SELECT count(*) FROM routes WHERE agency_id = $1),
                (SELECT count(*) FROM trips WHERE route_id IN (SELECT id FROM routes WHERE agency_id = $1)),
                (SELECT count(*) FROM stop_times WHERE trip_id IN (SELECT id FROM trips WHERE route_id IN (SELECT id FROM routes WHERE agency_id = $1))),
                COALESCE((SELECT max(created_at)::text FROM stops WHERE agency_id = $1), '')
        `, agency.ID)
		if err := row.Scan(&stats.Stops, &stats.Routes, &stats.Trips, &stats.StopTimes, &stats.LastSync); err != nil {
			return errInternal(c, err.Error())
		}

		c.Set("Cache-Control", "public, max-age=300")
		return c.JSON(fiber.Map{
			"agency": agency,
			"stats":  stats,
		})
	}
}

// RouteStopsHandler returns all stops served by a route, in order.
func RouteStopsHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return errBadRequest(c, "route id is required")
		}

		if deps.DB == nil {
			return errInternal(c, "database not available")
		}

		rows, err := deps.DB.Pool.Query(c.Context(), `
            SELECT DISTINCT ON (s.id)
                s.id, s.stop_id, s.name,
                ST_Y(s.location::geometry) as lat,
                ST_X(s.location::geometry) as lon,
                st.stop_sequence
            FROM stops s
            JOIN stop_times st ON st.stop_id = s.id
            JOIN trips t ON t.id = st.trip_id
            WHERE t.route_id = $1
            ORDER BY s.id, st.stop_sequence
        `, id)
		if err != nil {
			return errInternal(c, err.Error())
		}
		defer rows.Close()

		type routeStop struct {
			ID       string          `json:"id"`
			StopID   string          `json:"stop_id"`
			Name     string          `json:"name"`
			Location domain.GeoPoint `json:"location"`
			Sequence int             `json:"sequence"`
		}

		var stops []routeStop
		for rows.Next() {
			var s routeStop
			if err := rows.Scan(&s.ID, &s.StopID, &s.Name, &s.Location.Lat, &s.Location.Lon, &s.Sequence); err != nil {
				return errInternal(c, err.Error())
			}
			stops = append(stops, s)
		}

		c.Set("Cache-Control", "public, max-age=3600")
		return c.JSON(stops)
	}
}
