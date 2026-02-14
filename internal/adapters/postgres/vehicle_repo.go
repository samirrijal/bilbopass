package postgres

import (
	"context"
	"database/sql"

	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// VehiclePositionRepo implements ports.VehiclePositionRepository.
type VehiclePositionRepo struct {
	db *DB
}

func NewVehiclePositionRepo(db *DB) *VehiclePositionRepo {
	return &VehiclePositionRepo{db: db}
}

func (r *VehiclePositionRepo) Insert(ctx context.Context, vp *domain.VehiclePosition) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO vehicle_positions (time, vehicle_id, trip_id, route_id, location, bearing, speed, congestion_level, occupancy_status, metadata)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography, $7, $8, $9, $10, $11)
	`, vp.Time, vp.VehicleID, nilIfEmpty(vp.TripID), nilIfEmpty(vp.RouteID),
		vp.Location.Lon, vp.Location.Lat, vp.Bearing, vp.Speed,
		vp.CongestionLevel, vp.OccupancyStatus, vp.Metadata)
	return err
}

func (r *VehiclePositionRepo) InsertBatch(ctx context.Context, vps []domain.VehiclePosition) error {
	for _, vp := range vps {
		if err := r.Insert(ctx, &vp); err != nil {
			return err
		}
	}
	return nil
}

func (r *VehiclePositionRepo) LatestByRoute(ctx context.Context, routeID string) ([]domain.VehiclePosition, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT ON (vehicle_id)
			time, vehicle_id, trip_id, route_id,
			ST_Y(location::geometry) as lat,
			ST_X(location::geometry) as lon,
			bearing, speed, congestion_level, occupancy_status
		FROM vehicle_positions
		WHERE route_id = $1
		ORDER BY vehicle_id, time DESC
	`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []domain.VehiclePosition
	for rows.Next() {
		var vp domain.VehiclePosition
		var tripID, routeIDVal sql.NullString
		if err := rows.Scan(
			&vp.Time, &vp.VehicleID, &tripID, &routeIDVal,
			&vp.Location.Lat, &vp.Location.Lon,
			&vp.Bearing, &vp.Speed, &vp.CongestionLevel, &vp.OccupancyStatus,
		); err != nil {
			return nil, err
		}
		vp.TripID = tripID.String
		vp.RouteID = routeIDVal.String
		positions = append(positions, vp)
	}
	return positions, rows.Err()
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
