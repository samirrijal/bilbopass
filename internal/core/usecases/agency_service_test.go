package usecases_test

import (
	"context"
	"testing"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
)

// --- Mock AgencyRepository ---

type mockAgencyRepo struct {
	listFn      func(ctx context.Context) ([]domain.Agency, error)
	getBySlugFn func(ctx context.Context, slug string) (*domain.Agency, error)
}

func (m *mockAgencyRepo) Upsert(ctx context.Context, a *domain.Agency) error { return nil }

func (m *mockAgencyRepo) List(ctx context.Context) ([]domain.Agency, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, nil
}

func (m *mockAgencyRepo) GetBySlug(ctx context.Context, slug string) (*domain.Agency, error) {
	if m.getBySlugFn != nil {
		return m.getBySlugFn(ctx, slug)
	}
	return nil, nil
}

func TestAgencyService_List(t *testing.T) {
	repo := &mockAgencyRepo{
		listFn: func(ctx context.Context) ([]domain.Agency, error) {
			return []domain.Agency{
				{Slug: "metro_bilbao", Name: "Metro Bilbao"},
				{Slug: "bizkaibus", Name: "Bizkaibus"},
			}, nil
		},
	}

	svc := usecases.NewAgencyService(repo)
	agencies, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agencies) != 2 {
		t.Fatalf("expected 2 agencies, got %d", len(agencies))
	}
}

func TestAgencyService_GetBySlug(t *testing.T) {
	repo := &mockAgencyRepo{
		getBySlugFn: func(ctx context.Context, slug string) (*domain.Agency, error) {
			return &domain.Agency{Slug: slug, Name: "Metro Bilbao"}, nil
		},
	}

	svc := usecases.NewAgencyService(repo)
	a, err := svc.GetBySlug(context.Background(), "metro_bilbao")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Slug != "metro_bilbao" {
		t.Errorf("expected metro_bilbao, got %s", a.Slug)
	}
}
