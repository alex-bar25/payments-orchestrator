package webhook

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MaxAttempts = 5
	claimLimit  = 10
	lease       = 30 * time.Second
	pollEvery   = time.Second
)

type Worker struct {
	db     *gorm.DB
	client *http.Client
	url    string
	secret string
	logger *slog.Logger
}

func New(db *gorm.DB, url, secret string, logger *slog.Logger) *Worker {
	return &Worker{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
		url:    url,
		secret: secret,
		logger: logger,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.url == "" {
		w.logger.Info("WEBHOOK_URL empty; outbox will not be delivered")
	}
	t := time.NewTicker(pollEvery)
	defer t.Stop()
	for {
		if err := w.RunOnce(ctx); err != nil {
			w.logger.Error("webhook tick failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	if w.url == "" {
		return nil
	}
	now := time.Now().UTC()
	rows, err := w.claim(ctx, now)
	if err != nil {
		return err
	}
	for _, ev := range rows {
		err := Deliver(ctx, w.client, w.url, w.secret, ev)
		if err == nil {
			if err := w.finish(ctx, ev.ID, payment.WebhookDelivered, ev.Attempts, now, ""); err != nil {
				return err
			}
			continue
		}
		status := payment.WebhookPending
		next := now.Add(time.Second << uint(ev.Attempts))
		if ev.Attempts >= MaxAttempts {
			status = payment.WebhookFailed
			next = now
		}
		if err := w.finish(ctx, ev.ID, status, ev.Attempts, next, err.Error()); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) claim(ctx context.Context, now time.Time) ([]payment.WebhookEvent, error) {
	var rows []payment.WebhookEvent
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ? AND next_attempt_at <= ?", payment.WebhookPending, now).
			Order("next_attempt_at").
			Limit(claimLimit).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Find(&rows).Error; err != nil {
			return err
		}
		leaseUntil := now.Add(lease)
		for i := range rows {
			rows[i].Attempts++
			if err := tx.Model(&payment.WebhookEvent{}).Where("id = ?", rows[i].ID).Updates(map[string]any{
				"attempts":        rows[i].Attempts,
				"next_attempt_at": leaseUntil,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return rows, err
}

func (w *Worker) finish(ctx context.Context, id, status string, attempts int, next time.Time, lastErr string) error {
	return w.db.WithContext(ctx).Model(&payment.WebhookEvent{}).Where("id = ?", id).Updates(map[string]any{
		"status":          status,
		"attempts":        attempts,
		"next_attempt_at": next,
		"last_error":      lastErr,
	}).Error
}
