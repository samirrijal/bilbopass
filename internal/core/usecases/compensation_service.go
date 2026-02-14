package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
)

// CompensationService handles delay-compensation business logic.
type CompensationService struct {
	delays        ports.DelayEventRepository
	affiliates    ports.AffiliateRepository
	compensations ports.CompensationRepository
	notifier      ports.NotificationService
}

// NewCompensationService creates a new CompensationService.
func NewCompensationService(
	delays ports.DelayEventRepository,
	affiliates ports.AffiliateRepository,
	compensations ports.CompensationRepository,
	notifier ports.NotificationService,
) *CompensationService {
	return &CompensationService{
		delays:        delays,
		affiliates:    affiliates,
		compensations: compensations,
		notifier:      notifier,
	}
}

// IssueCompensation finds the nearest affiliate and creates a coupon for the user.
func (s *CompensationService) IssueCompensation(ctx context.Context, userID string, delayEvent *domain.DelayEvent, stopLat, stopLon float64) (*domain.Compensation, error) {
	// Find nearest active affiliate
	affiliates, err := s.affiliates.FindNearby(ctx, stopLat, stopLon, 5)
	if err != nil {
		return nil, fmt.Errorf("find affiliates: %w", err)
	}
	if len(affiliates) == 0 {
		return nil, fmt.Errorf("no affiliates found near stop")
	}

	affiliate := affiliates[0]

	// Generate unique coupon code
	code, err := generateCode()
	if err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}

	comp := &domain.Compensation{
		UserID:       userID,
		DelayEventID: delayEvent.ID,
		AffiliateID:  affiliate.ID,
		Code:         code,
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(72 * time.Hour),
	}

	if err := s.compensations.Create(ctx, comp); err != nil {
		return nil, fmt.Errorf("create compensation: %w", err)
	}

	// Mark the delay event as compensated
	if err := s.delays.MarkCompensated(ctx, delayEvent.ID); err != nil {
		// Best-effort; coupon already created
		_ = err
	}

	// Send push notification (best-effort)
	title := "Free coffee — sorry for the delay!"
	body := fmt.Sprintf("Show code %s at %s. Valid for 72 hours.", code, affiliate.Name)
	_ = s.notifier.SendPush(ctx, userID, title, body)

	return comp, nil
}

// RedeemCompensation marks a coupon as redeemed.
func (s *CompensationService) RedeemCompensation(ctx context.Context, code string) error {
	return s.compensations.Redeem(ctx, code)
}

func generateCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "BP-" + hex.EncodeToString(b), nil
}
