package recon

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const pollEvery = 5 * time.Second

type Worker struct {
	db     *gorm.DB
	logger *slog.Logger
}

func New(db *gorm.DB, logger *slog.Logger) *Worker {
	return &Worker{db: db, logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	t := time.NewTicker(pollEvery)
	defer t.Stop()
	for {
		if err := w.RunOnce(ctx); err != nil {
			w.logger.Error("recon tick failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	var payments []payment.Payment
	if err := w.db.WithContext(ctx).Find(&payments).Error; err != nil {
		return err
	}
	var rows []payment.ProviderLedger
	if err := w.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}
	ledgers := make(map[string]payment.ProviderLedger, len(rows))
	for _, row := range rows {
		ledgers[row.PaymentID] = row
	}
	now := time.Now().UTC()
	for _, m := range FindMismatches(payments, ledgers) {
		id, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		err = w.db.WithContext(ctx).Create(&payment.Discrepancy{
			ID:             "dsc_" + id.String(),
			PaymentID:      m.PaymentID,
			InternalStatus: m.Internal,
			ProviderStatus: m.Provider,
			CreatedAt:      now,
		}).Error
		if isUnique(err) {
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
