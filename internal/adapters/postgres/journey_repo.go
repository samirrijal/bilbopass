package postgres

import (
	"context"
	"time"

	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// JourneyRepo implements ports.JourneyRepository.
type JourneyRepo struct {
	db *DB
}

func NewJourneyRepo(db *DB) *JourneyRepo {
	return &JourneyRepo{db: db}
}

// FindJourneys finds possible journeys between two stops.
// It uses a two-phase approach:
//  1. Find direct trips (single leg, no transfers)
//  2. Find 1-transfer connections via shared intermediate stops
func (r *JourneyRepo) FindJourneys(ctx context.Context, fromStopID, toStopID string, departAfter time.Time, maxTransfers int, limit int) ([]domain.Journey, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	todSeconds := departAfter.Hour()*3600 + departAfter.Minute()*60 + departAfter.Second()
	today := time.Date(departAfter.Year(), departAfter.Month(), departAfter.Day(), 0, 0, 0, 0, departAfter.Location())

	var journeys []domain.Journey

	// Phase 1: Direct journeys (same trip serves both stops, from before to in sequence)
	directRows, err := r.db.Pool.Query(ctx, `
        SELECT
            st_from.departure_time AS dep_time,
            st_to.arrival_time AS arr_time,
            t.id AS trip_id, t.trip_id AS trip_code, COALESCE(t.headsign, '') AS headsign, COALESCE(t.direction_id, 0),
            r.id AS route_id, r.route_id AS route_code, COALESCE(r.short_name, '') AS short_name,
            r.long_name, r.route_type, r.color, r.text_color,
            fs.id AS from_stop_uuid, fs.stop_id AS from_stop_code, fs.name AS from_stop_name,
            ST_Y(fs.location::geometry), ST_X(fs.location::geometry),
            ts.id AS to_stop_uuid, ts.stop_id AS to_stop_code, ts.name AS to_stop_name,
            ST_Y(ts.location::geometry), ST_X(ts.location::geometry)
        FROM stop_times st_from
        JOIN stop_times st_to ON st_from.trip_id = st_to.trip_id
        JOIN trips t ON t.id = st_from.trip_id
        JOIN routes r ON r.id = t.route_id
        JOIN stops fs ON fs.id = st_from.stop_id
        JOIN stops ts ON ts.id = st_to.stop_id
        WHERE st_from.stop_id = $1
          AND st_to.stop_id = $2
          AND st_from.stop_sequence < st_to.stop_sequence
          AND st_from.departure_time >= make_interval(secs => $3)
        ORDER BY st_from.departure_time
        LIMIT $4
    `, fromStopID, toStopID, todSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer directRows.Close()

	for directRows.Next() {
		var depInterval, arrInterval time.Duration
		var trip domain.Trip
		var route domain.Route
		var fromStop, toStop domain.Stop
		var directionID int

		if err := directRows.Scan(
			&depInterval, &arrInterval,
			&trip.ID, &trip.TripID, &trip.Headsign, &directionID,
			&route.ID, &route.RouteID, &route.ShortName,
			&route.LongName, &route.RouteType, &route.Color, &route.TextColor,
			&fromStop.ID, &fromStop.StopID, &fromStop.Name,
			&fromStop.Location.Lat, &fromStop.Location.Lon,
			&toStop.ID, &toStop.StopID, &toStop.Name,
			&toStop.Location.Lat, &toStop.Location.Lon,
		); err != nil {
			return nil, err
		}

		trip.DirectionID = directionID
		trip.RouteID = route.ID
		depTime := today.Add(depInterval)
		arrTime := today.Add(arrInterval)
		duration := arrInterval - depInterval

		journeys = append(journeys, domain.Journey{
			Legs: []domain.JourneyLeg{
				{
					Route:    &route,
					FromStop: &fromStop,
					ToStop:   &toStop,
					Departure: domain.Departure{
						Trip:          &trip,
						ScheduledTime: depTime,
					},
					ArrivalTime: arrTime,
				},
			},
			Duration:      duration,
			DepartureTime: depTime,
			ArrivalTime:   arrTime,
			Transfers:     0,
		})
	}
	if err := directRows.Err(); err != nil {
		return nil, err
	}

	// Phase 2: 1-transfer journeys (if maxTransfers >= 1 and we have room)
	if maxTransfers >= 1 && len(journeys) < limit {
		remaining := limit - len(journeys)
		transferRows, err := r.db.Pool.Query(ctx, `
            WITH leg1 AS (
                SELECT
                    st1_from.stop_id AS from_stop,
                    st1_to.stop_id AS transfer_stop,
                    st1_from.departure_time AS dep1,
                    st1_to.arrival_time AS arr1,
                    st1_from.trip_id AS trip1_id
                FROM stop_times st1_from
                JOIN stop_times st1_to ON st1_from.trip_id = st1_to.trip_id
                    AND st1_from.stop_sequence < st1_to.stop_sequence
                WHERE st1_from.stop_id = $1
                  AND st1_from.departure_time >= make_interval(secs => $3)
            ),
            leg2 AS (
                SELECT
                    st2_from.stop_id AS transfer_stop,
                    st2_to.stop_id AS to_stop,
                    st2_from.departure_time AS dep2,
                    st2_to.arrival_time AS arr2,
                    st2_from.trip_id AS trip2_id
                FROM stop_times st2_from
                JOIN stop_times st2_to ON st2_from.trip_id = st2_to.trip_id
                    AND st2_from.stop_sequence < st2_to.stop_sequence
                WHERE st2_to.stop_id = $2
            )
            SELECT
                l1.dep1, l1.arr1, l2.dep2, l2.arr2,
                l1.transfer_stop,
                l1.trip1_id, l2.trip2_id,
                t1.trip_id, COALESCE(t1.headsign, ''),
                r1.id, r1.route_id, COALESCE(r1.short_name,''), r1.long_name, r1.color, r1.text_color, r1.route_type,
                t2.trip_id, COALESCE(t2.headsign, ''),
                r2.id, r2.route_id, COALESCE(r2.short_name,''), r2.long_name, r2.color, r2.text_color, r2.route_type,
                fs.id, fs.stop_id, fs.name, ST_Y(fs.location::geometry), ST_X(fs.location::geometry),
                xs.id, xs.stop_id, xs.name, ST_Y(xs.location::geometry), ST_X(xs.location::geometry),
                ds.id, ds.stop_id, ds.name, ST_Y(ds.location::geometry), ST_X(ds.location::geometry)
            FROM leg1 l1
            JOIN leg2 l2 ON l1.transfer_stop = l2.transfer_stop
                AND l2.dep2 >= l1.arr1 + interval '2 minutes'
                AND l2.dep2 <= l1.arr1 + interval '30 minutes'
            JOIN trips t1 ON t1.id = l1.trip1_id
            JOIN routes r1 ON r1.id = t1.route_id
            JOIN trips t2 ON t2.id = l2.trip2_id
            JOIN routes r2 ON r2.id = t2.route_id
            JOIN stops fs ON fs.id = l1.from_stop
            JOIN stops xs ON xs.id = l1.transfer_stop
            JOIN stops ds ON ds.id = l2.to_stop
            WHERE r1.id != r2.id
            ORDER BY l2.arr2 - l1.dep1
            LIMIT $4
        `, fromStopID, toStopID, todSeconds, remaining)
		if err != nil {
			// Transfer query is optional — log and continue with direct results
			return journeys, nil
		}
		defer transferRows.Close()

		for transferRows.Next() {
			var dep1, arr1, dep2, arr2 time.Duration
			var transferStopID string
			var trip1UUID, trip2UUID string
			var trip1Code, trip1Headsign string
			var r1 domain.Route
			var trip2Code, trip2Headsign string
			var r2 domain.Route
			var fromStop, xferStop, toStop domain.Stop
			var r1Type, r2Type int

			if err := transferRows.Scan(
				&dep1, &arr1, &dep2, &arr2,
				&transferStopID,
				&trip1UUID, &trip2UUID,
				&trip1Code, &trip1Headsign,
				&r1.ID, &r1.RouteID, &r1.ShortName, &r1.LongName, &r1.Color, &r1.TextColor, &r1Type,
				&trip2Code, &trip2Headsign,
				&r2.ID, &r2.RouteID, &r2.ShortName, &r2.LongName, &r2.Color, &r2.TextColor, &r2Type,
				&fromStop.ID, &fromStop.StopID, &fromStop.Name, &fromStop.Location.Lat, &fromStop.Location.Lon,
				&xferStop.ID, &xferStop.StopID, &xferStop.Name, &xferStop.Location.Lat, &xferStop.Location.Lon,
				&toStop.ID, &toStop.StopID, &toStop.Name, &toStop.Location.Lat, &toStop.Location.Lon,
			); err != nil {
				continue
			}

			r1.RouteType = r1Type
			r2.RouteType = r2Type

			t1 := &domain.Trip{ID: trip1UUID, TripID: trip1Code, Headsign: trip1Headsign, RouteID: r1.ID}
			t2 := &domain.Trip{ID: trip2UUID, TripID: trip2Code, Headsign: trip2Headsign, RouteID: r2.ID}

			depTime := today.Add(dep1)
			arrTime := today.Add(arr2)

			journeys = append(journeys, domain.Journey{
				Legs: []domain.JourneyLeg{
					{
						Route:    &r1,
						FromStop: &fromStop,
						ToStop:   &xferStop,
						Departure: domain.Departure{
							Trip:          t1,
							ScheduledTime: depTime,
						},
						ArrivalTime: today.Add(arr1),
					},
					{
						Route:    &r2,
						FromStop: &xferStop,
						ToStop:   &toStop,
						Departure: domain.Departure{
							Trip:          t2,
							ScheduledTime: today.Add(dep2),
						},
						ArrivalTime: arrTime,
					},
				},
				Duration:      arr2 - dep1,
				DepartureTime: depTime,
				ArrivalTime:   arrTime,
				Transfers:     1,
			})
		}
	}

	return journeys, nil
}
