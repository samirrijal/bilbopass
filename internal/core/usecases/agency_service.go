package usecases

import (
	"context"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
)

// AgencyService handles agency-related business logic.
type AgencyService struct {
	agencies ports.AgencyRepository
}

// NewAgencyService creates a new AgencyService.
func NewAgencyService(agencies ports.AgencyRepository) *AgencyService {
	return &AgencyService{agencies: agencies}
}

// List returns all agencies.
func (s *AgencyService) List(ctx context.Context) ([]domain.Agency, error) {
	return s.agencies.List(ctx)
}

// GetBySlug returns an agency by slug.
func (s *AgencyService) GetBySlug(ctx context.Context, slug string) (*domain.Agency, error) {
	return s.agencies.GetBySlug(ctx, slug)
}
