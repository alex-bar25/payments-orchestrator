package payment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/psp"
	"github.com/google/uuid"
	"golang.org/x/text/currency"
)

const (
	DemoMerchantID  = "merch_demo"
	EventCreated    = "payment.created"
	EventProcessing = "payment.processing"
	EventAuthorized = "payment.authorized"
	EventFailed     = "payment.failed"
	EventCaptured   = "payment.captured"
	EventRefunded   = "payment.refunded"
	EventCancelled  = "payment.cancelled"
)

var (
	ErrInvalidAmount           = errors.New("amount must be greater than zero")
	ErrInvalidCurrency         = errors.New("currency must be a valid ISO 4217 code")
	ErrInvalidIdempotencyKey   = errors.New("idempotency key is required and must be at most 255 characters")
	ErrIdempotencyKeyReuse     = errors.New("idempotency key reused with a different request")
	ErrNotFound                = errors.New("payment not found")
	ErrInvalidPaymentState     = errors.New("payment cannot be transitioned from its current state")
	ErrCaptureDeclined         = errors.New("capture was declined by the provider")
	ErrRefundDeclined          = errors.New("refund was declined by the provider")
	errDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
	errStaleVersion            = errors.New("payment version conflict")
)

type Payment struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	MerchantID        string    `json:"merchant_id"`
	Amount            int64     `json:"amount"`
	Currency          string    `json:"currency"`
	Status            Status    `json:"status"`
	Version           int       `json:"-"`
	ProviderPaymentID *string   `json:"provider_payment_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (Payment) TableName() string { return "payments" }

type Event struct {
	ID        string `gorm:"primaryKey"`
	PaymentID string
	Type      string
	Metadata  string `gorm:"type:jsonb"`
	RequestID string
	CreatedAt time.Time
}

func (Event) TableName() string { return "payment_events" }

type IdempotencyKey struct {
	MerchantID  string `gorm:"primaryKey"`
	Key         string `gorm:"primaryKey"`
	RequestHash string
	PaymentID   string
}

func (IdempotencyKey) TableName() string { return "idempotency_keys" }

type CreateInput struct {
	MerchantID     string
	Amount         int64
	Currency       string
	IdempotencyKey string
	RequestID      string
}

type Store interface {
	InsertCreated(ctx context.Context, p Payment, e Event, key IdempotencyKey) error
	LookupIdempotency(ctx context.Context, merchantID, key string) (IdempotencyKey, error)
	Get(ctx context.Context, id string) (Payment, error)
	SaveTransition(ctx context.Context, p Payment, from Status, e Event, key *IdempotencyKey) error
}

type Service struct {
	store Store
	psp   psp.Mock
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Payment, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" || len(key) > 255 {
		return Payment{}, ErrInvalidIdempotencyKey
	}
	if in.Amount <= 0 {
		return Payment{}, ErrInvalidAmount
	}
	iso := strings.ToUpper(strings.TrimSpace(in.Currency))
	if _, err := currency.ParseISO(iso); err != nil {
		return Payment{}, ErrInvalidCurrency
	}

	now := time.Now().UTC()
	payUUID, err := uuid.NewRandom()
	if err != nil {
		return Payment{}, err
	}
	p := Payment{
		ID:         "pay_" + payUUID.String(),
		MerchantID: in.MerchantID,
		Amount:     in.Amount,
		Currency:   iso,
		Status:     StatusRequiresPayment,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	evtUUID, err := uuid.NewRandom()
	if err != nil {
		return Payment{}, err
	}
	e := Event{
		ID:        "evt_" + evtUUID.String(),
		PaymentID: p.ID,
		Type:      EventCreated,
		Metadata:  "{}",
		RequestID: in.RequestID,
		CreatedAt: now,
	}
	fingerprint, err := json.Marshal(struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	}{in.Amount, iso})
	if err != nil {
		return Payment{}, err
	}

	err = s.store.InsertCreated(ctx, p, e, IdempotencyKey{
		MerchantID:  in.MerchantID,
		Key:         key,
		RequestHash: string(fingerprint),
		PaymentID:   p.ID,
	})
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, errDuplicateIdempotencyKey) {
		return Payment{}, err
	}

	rec, err := s.store.LookupIdempotency(ctx, in.MerchantID, key)
	if err != nil {
		return Payment{}, err
	}
	if rec.RequestHash != string(fingerprint) {
		return Payment{}, ErrIdempotencyKeyReuse
	}
	return s.store.Get(ctx, rec.PaymentID)
}
