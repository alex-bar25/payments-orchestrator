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

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
