package delivery

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/shohag/piperelay/internal/models"
	"github.com/shohag/piperelay/internal/storage"
)

type Worker struct {
	store         storage.Storage
	sender        *Sender
	maxAttempts   int
	retrySchedule []time.Duration
	log           zerolog.Logger
}

func NewWorker(store storage.Storage, sender *Sender, maxAttempts int, retrySchedule []time.Duration, log zerolog.Logger) *Worker {
	return &Worker{
		store:         store,
		sender:        sender,
		maxAttempts:   maxAttempts,
		retrySchedule: retrySchedule,
		log:           log,
	}
}

func (w *Worker) Process(ctx context.Context, d models.Delivery) {
	msg, err := w.store.GetMessage(ctx, d.MessageID)
	if err != nil || msg == nil {
		w.log.Error().Err(err).Str("delivery_id", d.ID).Msg("failed to get message for delivery")
		return
	}

	ep, err := w.store.GetEndpoint(ctx, d.EndpointID)
	if err != nil || ep == nil {
		w.log.Error().Err(err).Str("delivery_id", d.ID).Msg("failed to get endpoint for delivery")
		return
	}

	if !ep.Active {
		w.log.Info().Str("delivery_id", d.ID).Msg("skipping delivery to inactive endpoint")
		return
	}

	result := w.sender.Send(ctx, ep.URL, ep.Secret, msg.ID, msg.Payload)

	d.AttemptCount++
	now := time.Now().UTC()

	attempt := &models.Attempt{
		ID:            models.NewID("att"),
		DeliveryID:    d.ID,
		AttemptNumber: d.AttemptCount,
		StatusCode:    result.StatusCode,
		ResponseBody:  result.ResponseBody,
		LatencyMs:     result.LatencyMs,
		Error:         result.Error,
		CreatedAt:     now.Format(time.RFC3339),
	}

	if err := w.store.CreateAttempt(ctx, attempt); err != nil {
		w.log.Error().Err(err).Str("delivery_id", d.ID).Msg("failed to record attempt")
	}

	if result.Error == "" && IsSuccess(result.StatusCode) {
		d.Status = models.DeliverySuccess
		d.NextRetryAt = nil
		w.log.Info().
			Str("delivery_id", d.ID).
			Int("status_code", result.StatusCode).
			Int64("latency_ms", result.LatencyMs).
			Msg("delivery succeeded")
	} else if d.AttemptCount >= w.maxAttempts {
		d.Status = models.DeliveryFailed
		d.NextRetryAt = nil
		w.log.Warn().
			Str("delivery_id", d.ID).
			Int("attempts", d.AttemptCount).
			Str("error", result.Error).
			Msg("delivery permanently failed")
	} else {
		d.Status = models.DeliveryRetrying
		d.NextRetryAt = NextRetryTime(d.AttemptCount, w.retrySchedule)
		w.log.Info().
			Str("delivery_id", d.ID).
			Int("attempt", d.AttemptCount).
			Time("next_retry", *d.NextRetryAt).
			Msg("delivery scheduled for retry")
	}

	if err := w.store.UpdateDelivery(ctx, &d); err != nil {
		w.log.Error().Err(err).Str("delivery_id", d.ID).Msg("failed to update delivery")
	}
}
