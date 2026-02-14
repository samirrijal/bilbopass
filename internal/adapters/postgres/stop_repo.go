package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// StopRepo implements ports.StopRepository with pgx.
type StopRepo struct {
	db *DB
}

// NewStopRepo creates a new StopRepo.
func NewStopRepo(db *DB) *StopRepo {
	return &StopRepo{db: db}
}

// Upsert inserts or updates a single stop.
func (r *StopRepo) Upsert(ctx context.Context, s *domain.Stop) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO stops (stop_id, agency_id, name, location, platform_code, wheelchair_accessible, metadata)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, $7, $8)
		ON CONFLICT (agency_id, stop_id) DO UPDATE
		SET name = EXCLUDED.name, location = EXCLUDED.location,
		    platform_code = EXCLUDED.platform_code,
		    wheelchair_accessible = EXCLUDED.wheelchair_accessible,
		    metadata = EXCLUDED.metadata
	`, s.StopID, s.AgencyID, s.Name, s.Location.Lon, s.Location.Lat,
		s.PlatformCode, s.WheelchairAccessible, s.Metadata)
	return err
}

// UpsertBatch inserts many stops using pgx.Batch.
func (r *StopRepo) UpsertBatch(ctx context.Context, stops []domain.Stop) error {
	batch := &pgx.Batch{}
	for _, s := range stops {
		batch.Queue(`
			INSERT INTO stops (stop_id, agency_id, name, location, platform_code, wheelchair_accessible, metadata)
			VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, $7, $8)
			ON CONFLICT (agency_id, stop_id) DO UPDATE
			SET name = EXCLUDED.name, location = EXCLUDED.location
		`, s.StopID, s.AgencyID, s.Name, s.Location.Lon, s.Location.Lat,
			s.PlatformCode, s.WheelchairAccessible, s.Metadata)
	}
	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()
	for range stops {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch exec: %w", err)
		}
	}
	return nil
}

// GetByID returns a stop by UUID.
func (r *StopRepo) GetByID(ctx context.Context, id string) (*domain.Stop, error) {
	var s domain.Stop
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, stop_id, agency_id, name,
		       ST_Y(location::geometry) as lat,
		       ST_X(location::geometry) as lon,
		       COALESCE(platform_code, ''), wheelchair_accessible, COALESCE(metadata, '{}'), created_at
		FROM stops WHERE id = $1
	`, id).Scan(
		&s.ID, &s.StopID, &s.AgencyID, &s.Name,
		&s.Location.Lat, &s.Location.Lon,
		&s.PlatformCode, &s.WheelchairAccessible, &s.Metadata, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetByIDs returns multiple stops by UUID, in arbitrary order.
func (r *StopRepo) GetByIDs(ctx context.Context, ids []string) ([]domain.Stop, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, stop_id, agency_id, name,
		       ST_Y(location::geometry) as lat,
		       ST_X(location::geometry) as lon,
		       COALESCE(platform_code, ''), wheelchair_accessible, COALESCE(metadata, '{}'), created_at
		FROM stops WHERE id = ANY($1)
		ORDER BY name
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stops []domain.Stop
	for rows.Next() {
		var s domain.Stop
		if err := rows.Scan(
			&s.ID, &s.StopID, &s.AgencyID, &s.Name,
			&s.Location.Lat, &s.Location.Lon,
			&s.PlatformCode, &s.WheelchairAccessible, &s.Metadata, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		stops = append(stops, s)
	}
	return stops, rows.Err()
}

// FindNearby returns stops within radiusMeters using PostGIS ST_DWithin.
func (r *StopRepo) FindNearby(ctx context.Context, lat, lon, radiusMeters float64, limit int) ([]domain.Stop, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, stop_id, agency_id, name,
		       ST_Y(location::geometry) as lat,
		       ST_X(location::geometry) as lon,
		       COALESCE(platform_code, ''), wheelchair_accessible,
		       ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) as distance,
		       created_at
		FROM stops
		WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance
		LIMIT $4
	`, lon, lat, radiusMeters, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stops []domain.Stop
	for rows.Next() {
		var s domain.Stop
		var dist float64
		if err := rows.Scan(
			&s.ID, &s.StopID, &s.AgencyID, &s.Name,
			&s.Location.Lat, &s.Location.Lon,
			&s.PlatformCode, &s.WheelchairAccessible,
			&dist, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		s.Distance = &dist
		stops = append(stops, s)
	}
	return stops, rows.Err()
}

// Search performs fuzzy + full-text search on stop names.
func (r *StopRepo) Search(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, stop_id, agency_id, name,
		       ST_Y(location::geometry) as lat,
		       ST_X(location::geometry) as lon,
		       COALESCE(platform_code, ''), wheelchair_accessible, created_at,
		       similarity(name, $1) as sim
		FROM stops
		WHERE name_vector @@ plainto_tsquery('spanish', $1)
		   OR name %> $1
		ORDER BY sim DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stops []domain.Stop
	for rows.Next() {
		var s domain.Stop
		var sim float64
		if err := rows.Scan(
			&s.ID, &s.StopID, &s.AgencyID, &s.Name,
			&s.Location.Lat, &s.Location.Lon,
			&s.PlatformCode, &s.WheelchairAccessible, &s.CreatedAt,
			&sim,
		); err != nil {
			return nil, err
		}
		stops = append(stops, s)
	}
	return stops, rows.Err()
}
