package payment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(db *gorm.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) InsertCreated(ctx context.Context, p Payment, e Event, key IdempotencyKey) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		if err := tx.Create(&e).Error; err != nil {
			return err
		}
		if err := insertOutbox(tx, p, e); err != nil {
			return err
		}
		err := tx.Create(&key).Error
		if isUniqueViolation(err, "idempotency_keys_pkey") {
			return errDuplicateIdempotencyKey
		}
		return err
	})
}

func (s *PostgresStore) LookupIdempotency(ctx context.Context, merchantID, key string) (IdempotencyKey, error) {
	var row IdempotencyKey
	err := s.db.WithContext(ctx).Where("merchant_id = ? AND key = ?", merchantID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IdempotencyKey{}, ErrNotFound
	}
	return row, err
}

func (s *PostgresStore) Get(ctx context.Context, id string) (Payment, error) {
	var p Payment
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Payment{}, ErrNotFound
	}
	return p, err
}

func (s *PostgresStore) ListEvents(ctx context.Context, paymentID string) ([]Event, error) {
	var events []Event
	err := s.db.WithContext(ctx).Where("payment_id = ?", paymentID).Order("created_at, id").Find(&events).Error
	return events, err
}

func (s *PostgresStore) SaveTransition(ctx context.Context, p Payment, from Status, e Event, key *IdempotencyKey, pe *ProviderEvent) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":     p.Status,
			"version":    p.Version,
			"updated_at": p.UpdatedAt,
		}
		if p.ProviderPaymentID != nil {
			updates["provider_payment_id"] = *p.ProviderPaymentID
		}
		res := tx.Model(&Payment{}).Where("id = ? AND status = ? AND version = ?", p.ID, from, p.Version-1).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errStaleVersion
		}
		if err := tx.Create(&e).Error; err != nil {
			return err
		}
		if err := insertOutbox(tx, p, e); err != nil {
			return err
		}
		if pe != nil {
			err := tx.Create(pe).Error
			if isUniqueViolation(err, "provider_events_pkey") {
				return errDuplicateProviderEvent
			}
			if err != nil {
				return err
			}
		}
		if key == nil {
			return nil
		}
		err := tx.Create(key).Error
		if isUniqueViolation(err, "idempotency_keys_pkey") {
			return errDuplicateIdempotencyKey
		}
		return err
	})
}

func (s *PostgresStore) LookupProviderEvent(ctx context.Context, id string) (ProviderEvent, error) {
	var row ProviderEvent
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ProviderEvent{}, ErrNotFound
	}
	return row, err
}

func (s *PostgresStore) InsertProviderEvent(ctx context.Context, pe ProviderEvent) error {
	err := s.db.WithContext(ctx).Create(&pe).Error
	if isUniqueViolation(err, "provider_events_pkey") {
		return errDuplicateProviderEvent
	}
	return err
}

func insertOutbox(tx *gorm.DB, p Payment, e Event) error {
	wh, ok, err := newOutboxEvent(p, e)
	if err != nil || !ok {
		return err
	}
	return tx.Create(&wh).Error
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
