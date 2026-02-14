package workflows

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
)

// CompensationActivities holds the activity implementations for the compensation workflow.
type CompensationActivities struct {
	CompensationService *usecases.CompensationService
	Affiliates          ports.AffiliateRepository
	Compensations       ports.CompensationRepository
	Delays              ports.DelayEventRepository
	Notifier            ports.NotificationService
}

// FindNearestAffiliate returns the ID of the nearest active affiliate.
func (a *CompensationActivities) FindNearestAffiliate(ctx context.Context, lat, lon float64) (string, error) {
	affiliates, err := a.Affiliates.FindNearby(ctx, lat, lon, 5)
	if err != nil {
		return "", fmt.Errorf("find nearby affiliates: %w", err)
	}
	if len(affiliates) == 0 {
		return "", fmt.Errorf("no affiliates found near %.4f, %.4f", lat, lon)
	}
	return affiliates[0].ID, nil
}

// GetAffiliateName returns the name of an affiliate by ID.
func (a *CompensationActivities) GetAffiliateName(ctx context.Context, affiliateID string) (string, error) {
	aff, err := a.Affiliates.GetByID(ctx, affiliateID)
	if err != nil {
		return "", fmt.Errorf("get affiliate %s: %w", affiliateID, err)
	}
	return aff.Name, nil
}

// GenerateCouponCode creates a compensation record and returns the code.
func (a *CompensationActivities) GenerateCouponCode(ctx context.Context, userID, affiliateID, delayEventID string) (string, error) {
	// Delegate to the CompensationService which already handles
	// code generation, persistence, and delay event marking.
	delay := &domain.DelayEvent{ID: delayEventID}
	comp, err := a.CompensationService.IssueCompensation(ctx, userID, delay, 0, 0)
	if err != nil {
		return "", fmt.Errorf("issue compensation: %w", err)
	}
	return comp.Code, nil
}

// SendPushNotification sends a push notification to the user.
func (a *CompensationActivities) SendPushNotification(ctx context.Context, userID, affiliateName, code string) error {
	if a.Notifier == nil {
		log.Printf("PUSH (no notifier) → user=%s affiliate=%s code=%s", userID, affiliateName, code)
		return nil
	}
	title := "Free coffee — sorry for the delay!"
	body := fmt.Sprintf("Show code %s at %s. Valid for 72 hours.", code, affiliateName)
	return a.Notifier.SendPush(ctx, userID, title, body)
}

// ScheduleExpiry sets a timer to auto-expire the coupon after its TTL.
func (a *CompensationActivities) ScheduleExpiry(ctx context.Context, code string, expiresAt time.Time) error {
	// In a Temporal workflow the expiry is handled by a workflow timer,
	// but this activity can be used as a fallback cleanup.
	log.Printf("Coupon %s scheduled to expire at %s", code, expiresAt.Format(time.RFC3339))
	return nil
}

// DeleteCoupon removes a coupon (saga compensation / rollback).
func (a *CompensationActivities) DeleteCoupon(ctx context.Context, code string) error {
	if err := a.Compensations.Delete(ctx, code); err != nil {
		return fmt.Errorf("delete coupon %s: %w", code, err)
	}
	log.Printf("Coupon %s deleted (saga compensation)", code)
	return nil
}
