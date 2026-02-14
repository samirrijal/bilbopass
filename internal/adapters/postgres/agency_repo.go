package postgres

import (
	"context"
	"database/sql"

	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// AgencyRepo implements ports.AgencyRepository.
type AgencyRepo struct {
	db *DB
}

func NewAgencyRepo(db *DB) *AgencyRepo {
	return &AgencyRepo{db: db}
}

func (r *AgencyRepo) Upsert(ctx context.Context, agency *domain.Agency) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO agencies (slug, name, url, timezone)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, url = EXCLUDED.url
	`, agency.Slug, agency.Name, agency.URL, agency.Timezone)
	return err
}

func (r *AgencyRepo) GetBySlug(ctx context.Context, slug string) (*domain.Agency, error) {
	a := &domain.Agency{}
	var urlVal sql.NullString
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, slug, name, COALESCE(url, ''), timezone, created_at
		FROM agencies WHERE slug = $1
	`, slug).Scan(&a.ID, &a.Slug, &a.Name, &urlVal, &a.Timezone, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.URL = urlVal.String
	return a, nil
}

func (r *AgencyRepo) List(ctx context.Context) ([]domain.Agency, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, slug, name, COALESCE(url, ''), timezone, created_at
		FROM agencies ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agencies []domain.Agency
	for rows.Next() {
		var a domain.Agency
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.URL, &a.Timezone, &a.CreatedAt); err != nil {
			return nil, err
		}
		agencies = append(agencies, a)
	}
	return agencies, rows.Err()
}
