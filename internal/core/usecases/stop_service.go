package usecases

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
)

// StopService handles stop-related business logic.
type StopService struct {
	stops ports.StopRepository
	cache ports.CacheService
}

// NewStopService creates a new StopService.
func NewStopService(stops ports.StopRepository, cache ports.CacheService) *StopService {
	return &StopService{stops: stops, cache: cache}
}

// FindNearby returns stops within radiusMeters of the given point.
func (s *StopService) FindNearby(ctx context.Context, lat, lon, radiusMeters float64, limit int) ([]domain.Stop, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	// Try cache
	cacheKey := fmt.Sprintf("stops:nearby:%.4f:%.4f:%.0f:%d", lat, lon, radiusMeters, limit)
	if s.cache != nil {
		if data, err := s.cache.Get(ctx, cacheKey); err == nil {
			var stops []domain.Stop
			if err := json.Unmarshal(data, &stops); err == nil {
				return stops, nil
			}
		}
	}

	stops, err := s.stops.FindNearby(ctx, lat, lon, radiusMeters, limit)
	if err != nil {
		return nil, err
	}

	// Cache for 5 minutes (stops don't change frequently)
	if s.cache != nil {
		if data, err := json.Marshal(stops); err == nil {
			_ = s.cache.Set(ctx, cacheKey, data, 300)
		}
	}

	return stops, nil
}

// Search performs fuzzy + full-text search on stop names.
func (s *StopService) Search(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error) {
	if query == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Try cache
	cacheKey := fmt.Sprintf("stops:search:%s:%d", query, limit)
	if s.cache != nil {
		if data, err := s.cache.Get(ctx, cacheKey); err == nil {
			var stops []domain.Stop
			if err := json.Unmarshal(data, &stops); err == nil {
				return stops, nil
			}
		}
	}

	stops, err := s.stops.Search(ctx, query, near, limit)
	if err != nil {
		return nil, err
	}

	// Cache for 5 minutes
	if s.cache != nil {
		if data, err := json.Marshal(stops); err == nil {
			_ = s.cache.Set(ctx, cacheKey, data, 300)
		}
	}

	return stops, nil
}

// GetByID returns a single stop.
func (s *StopService) GetByID(ctx context.Context, id string) (*domain.Stop, error) {
	cacheKey := "stops:id:" + id
	if s.cache != nil {
		if data, err := s.cache.Get(ctx, cacheKey); err == nil {
			var stop domain.Stop
			if err := json.Unmarshal(data, &stop); err == nil {
				return &stop, nil
			}
		}
	}

	stop, err := s.stops.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if data, err := json.Marshal(stop); err == nil {
			_ = s.cache.Set(ctx, cacheKey, data, 600) // 10 min for single stop
		}
	}

	return stop, nil
}

// GetByIDs returns multiple stops by their IDs.
func (s *StopService) GetByIDs(ctx context.Context, ids []string) ([]domain.Stop, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return s.stops.GetByIDs(ctx, ids)
}
