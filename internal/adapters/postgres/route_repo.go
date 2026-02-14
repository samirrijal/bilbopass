package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// RouteRepo implements ports.RouteRepository.
type RouteRepo struct {
	db *DB
}

func NewRouteRepo(db *DB) *RouteRepo { return &RouteRepo{db: db} }

func (r *RouteRepo) Upsert(ctx context.Context, route *domain.Route) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO routes (route_id, agency_id, short_name, long_name, route_type, color, text_color)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (agency_id, route_id) DO UPDATE
		SET short_name = EXCLUDED.short_name, long_name = EXCLUDED.long_name,
		    route_type = EXCLUDED.route_type, color = EXCLUDED.color, text_color = EXCLUDED.text_color
	`, route.RouteID, route.AgencyID, route.ShortName, route.LongName,
		route.RouteType, route.Color, route.TextColor)
	return err
}

func (r *RouteRepo) UpsertBatch(ctx context.Context, routes []domain.Route) error {
	batch := &pgx.Batch{}
	for _, rt := range routes {
		batch.Queue(`
			INSERT INTO routes (route_id, agency_id, short_name, long_name, route_type, color, text_color)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (agency_id, route_id) DO UPDATE
			SET short_name = EXCLUDED.short_name, long_name = EXCLUDED.long_name
		`, rt.RouteID, rt.AgencyID, rt.ShortName, rt.LongName,
			rt.RouteType, rt.Color, rt.TextColor)
	}
	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()
	for range routes {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch exec: %w", err)
		}
	}
	return nil
}

func (r *RouteRepo) GetByID(ctx context.Context, id string) (*domain.Route, error) {
	var rt domain.Route
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, route_id, agency_id, short_name, long_name, route_type, color, text_color, created_at
		FROM routes WHERE id = $1
	`, id).Scan(&rt.ID, &rt.RouteID, &rt.AgencyID, &rt.ShortName, &rt.LongName,
		&rt.RouteType, &rt.Color, &rt.TextColor, &rt.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *RouteRepo) ListByAgency(ctx context.Context, agencyID string) ([]domain.Route, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, route_id, agency_id, short_name, long_name, route_type, color, text_color, created_at
		FROM routes WHERE agency_id = $1 ORDER BY short_name
	`, agencyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []domain.Route
	for rows.Next() {
		var rt domain.Route
		if err := rows.Scan(&rt.ID, &rt.RouteID, &rt.AgencyID, &rt.ShortName, &rt.LongName,
			&rt.RouteType, &rt.Color, &rt.TextColor, &rt.CreatedAt); err != nil {
			return nil, err
		}
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}

// ListByStop returns the distinct routes that serve a given stop (via stop_times + trips).
func (r *RouteRepo) ListByStop(ctx context.Context, stopUUID string) ([]domain.Route, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT r.id, r.route_id, r.agency_id, r.short_name, r.long_name,
		       r.route_type, r.color, r.text_color, r.created_at
		FROM routes r
		JOIN trips t ON t.route_id = r.id
		JOIN stop_times st ON st.trip_id = t.id
		WHERE st.stop_id = $1
		ORDER BY r.short_name
	`, stopUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []domain.Route
	for rows.Next() {
		var rt domain.Route
		if err := rows.Scan(&rt.ID, &rt.RouteID, &rt.AgencyID, &rt.ShortName, &rt.LongName,
			&rt.RouteType, &rt.Color, &rt.TextColor, &rt.CreatedAt); err != nil {
			return nil, err
		}
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}
