package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CompensationInput is the input for the compensation workflow.
type CompensationInput struct {
	DelayEventID string
	UserID       string
	StopID       string
	StopLat      float64
	StopLon      float64
	DelayMinutes int
}

// CompensationWorkflow orchestrates finding an affiliate, generating a coupon,
// and sending a push notification. If the notification fails, the coupon is deleted
// (saga compensation).
func CompensationWorkflow(ctx workflow.Context, input CompensationInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting compensation workflow", "delayMinutes", input.DelayMinutes)

	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	// Step 1: Find nearest affiliate
	var affiliateID string
	var affiliateName string
	err := workflow.ExecuteActivity(ctx, "FindNearestAffiliate", input.StopLat, input.StopLon).Get(ctx, &affiliateID)
	if err != nil {
		return err
	}
	_ = workflow.ExecuteActivity(ctx, "GetAffiliateName", affiliateID).Get(ctx, &affiliateName)

	// Step 2: Generate coupon code
	var code string
	err = workflow.ExecuteActivity(ctx, "GenerateCouponCode", input.UserID, affiliateID, input.DelayEventID).Get(ctx, &code)
	if err != nil {
		return err
	}

	// Step 3: Send push notification
	err = workflow.ExecuteActivity(ctx, "SendPushNotification", input.UserID, affiliateName, code).Get(ctx, nil)
	if err != nil {
		logger.Warn("push notification failed, compensating", "error", err)
		// Compensate: delete the coupon
		_ = workflow.ExecuteActivity(ctx, "DeleteCoupon", code).Get(ctx, nil)
		return err
	}

	logger.Info("Compensation sent successfully", "code", code)
	return nil
}
