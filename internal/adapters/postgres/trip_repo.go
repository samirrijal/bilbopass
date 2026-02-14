package postgres

import (
	"context"
	"time"

	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// TripRepo implements ports.TripRepository.
type TripRepo struct {
	db *DB
}

func NewTripRepo(db *DB) *TripRepo {
	return &TripRepo{db: db}
}

func (r *TripRepo) Upsert(ctx context.Context, trip *domain.Trip) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO trips (trip_id, route_id, service_id, headsign, direction_id, shape_id, wheelchair_accessible, bikes_allowed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (route_id, trip_id) DO UPDATE
		SET headsign = EXCLUDED.headsign, direction_id = EXCLUDED.direction_id
	`, trip.TripID, trip.RouteID, trip.ServiceID, trip.Headsign, trip.DirectionID, trip.ShapeID, trip.WheelchairAccessible, trip.BikesAllowed)
	return err
}

func (r *TripRepo) UpsertBatch(ctx context.Context, trips []domain.Trip) error {
	for _, t := range trips {
		if err := r.Upsert(ctx, &t); err != nil {
			return err
		}
	}
	return nil
}

func (r *TripRepo) GetByID(ctx context.Context, id string) (*domain.Trip, error) {
	tr := &domain.Trip{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, trip_id, route_id, service_id, COALESCE(headsign, ''), COALESCE(direction_id, 0),
		       COALESCE(shape_id, ''), wheelchair_accessible, bikes_allowed, created_at
		FROM trips WHERE id = $1
	`, id).Scan(&tr.ID, &tr.TripID, &tr.RouteID, &tr.ServiceID, &tr.Headsign,
		&tr.DirectionID, &tr.ShapeID, &tr.WheelchairAccessible, &tr.BikesAllowed, &tr.CreatedAt)
	return tr, err
}

func (r *TripRepo) UpsertStopTimes(ctx context.Context, stopTimes []domain.StopTime) error {
	// Batch insert (used by ingestor, not API)
	return nil
}

func (r *TripRepo) GetStopTimes(ctx context.Context, tripID string) ([]domain.StopTime, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, trip_id, stop_id, arrival_time, departure_time, stop_sequence, pickup_type, drop_off_type, created_at
		FROM stop_times WHERE trip_id = $1 ORDER BY stop_sequence
	`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var times []domain.StopTime
	for rows.Next() {
		var st domain.StopTime
		if err := rows.Scan(&st.ID, &st.TripID, &st.StopID, &st.ArrivalTime, &st.DepartureTime,
			&st.StopSequence, &st.PickupType, &st.DropOffType, &st.CreatedAt); err != nil {
			return nil, err
		}
		times = append(times, st)
	}
	return times, rows.Err()
}

// NextDeparturesAtStop returns the next departures at a stop (schedule-based).
// It matches stop_times where the departure_time interval is >= current time-of-day.
func (r *TripRepo) NextDeparturesAtStop(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
	now := time.Now()
	// Current time of day as interval
	todSeconds := now.Hour()*3600 + now.Minute()*60 + now.Second()

	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			st.departure_time,
			t.id, t.trip_id, COALESCE(t.headsign, ''), COALESCE(t.direction_id, 0),
			r.id, r.route_id, COALESCE(r.short_name, ''), r.long_name, r.route_type, r.color, r.text_color
		FROM stop_times st
		JOIN trips t ON t.id = st.trip_id
		JOIN routes r ON r.id = t.route_id
		WHERE st.stop_id = $1
		  AND st.departure_time >= make_interval(secs => $2)
		ORDER BY st.departure_time
		LIMIT $3
	`, stopUUID, todSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var departures []domain.Departure
	for rows.Next() {
		var depInterval time.Duration
		var trip domain.Trip
		var route domain.Route

		if err := rows.Scan(
			&depInterval,
			&trip.ID, &trip.TripID, &trip.Headsign, &trip.DirectionID,
			&route.ID, &route.RouteID, &route.ShortName, &route.LongName, &route.RouteType, &route.Color, &route.TextColor,
		); err != nil {
			return nil, err
		}

		scheduledTime := today.Add(depInterval)

		departures = append(departures, domain.Departure{
			Trip:          &trip,
			ScheduledTime: scheduledTime,
			Platform:      "",
		})

		// Set route on trip for display
		trip.RouteID = route.ID
	}
	return departures, rows.Err()
}
