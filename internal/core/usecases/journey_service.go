package usecases

import (
    "context"
    "fmt"
    "time"

    "github.com/samirrijal/bilbopass/internal/core/domain"
    "github.com/samirrijal/bilbopass/internal/core/ports"
)

// JourneyService handles journey planning between stops.
type JourneyService struct {
    journeys ports.JourneyRepository
    stops    ports.StopRepository
}

// NewJourneyService creates a new JourneyService.
func NewJourneyService(journeys ports.JourneyRepository, stops ports.StopRepository) *JourneyService {
    return &JourneyService{journeys: journeys, stops: stops}
}

// PlanJourney finds routes between two stops.
func (s *JourneyService) PlanJourney(ctx context.Context, fromStopID, toStopID string, departAt *time.Time, maxTransfers int) ([]domain.Journey, error) {
    if fromStopID == "" || toStopID == "" {
        return nil, fmt.Errorf("from and to stop IDs are required")
    }
    if fromStopID == toStopID {
        return nil, fmt.Errorf("from and to stops must be different")
    }

    // Default departure time is now
    depTime := time.Now()
    if departAt != nil {
        depTime = *departAt
    }

    if maxTransfers < 0 || maxTransfers > 2 {
        maxTransfers = 1
    }

    return s.journeys.FindJourneys(ctx, fromStopID, toStopID, depTime, maxTransfers, 10)
}

// PlanJourneyByName finds stops by name first, then plans a journey.
func (s *JourneyService) PlanJourneyByName(ctx context.Context, fromName, toName string, departAt *time.Time) ([]domain.Journey, error) {
    fromStops, err := s.stops.Search(ctx, fromName, nil, 1)
    if err != nil || len(fromStops) == 0 {
        return nil, fmt.Errorf("origin stop not found: %s", fromName)
    }

    toStops, err := s.stops.Search(ctx, toName, nil, 1)
    if err != nil || len(toStops) == 0 {
        return nil, fmt.Errorf("destination stop not found: %s", toName)
    }

    return s.PlanJourney(ctx, fromStops[0].ID, toStops[0].ID, departAt, 1)
}
