package payment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) Cancel(ctx context.Context, merchantID, paymentID, idempotencyKey, requestID string) (Payment, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 255 {
		return Payment{}, ErrInvalidIdempotencyKey
	}
	fingerprint, err := json.Marshal(struct {
		Op        string `json:"op"`
		PaymentID string `json:"payment_id"`
	}{"cancel", paymentID})
	if err != nil {
		return Payment{}, err
	}

	rec, err := s.store.LookupIdempotency(ctx, merchantID, key)
	if err == nil {
		if rec.RequestHash != string(fingerprint) {
			return Payment{}, ErrIdempotencyKeyReuse
		}
		return s.store.Get(ctx, rec.PaymentID)
	}
	if !errors.Is(err, ErrNotFound) {
		return Payment{}, err
	}

	p, err := s.store.Get(ctx, paymentID)
	if err != nil {
		return Payment{}, err
	}
	if p.Status == StatusCancelled {
		return p, nil
	}
	if p.Status != StatusAuthorized {
		return Payment{}, ErrInvalidPaymentState
	}

	evtUUID, err := uuid.NewRandom()
	if err != nil {
		return Payment{}, err
	}
	from := p.Status
	p.Status = StatusCancelled
	p.Version++
	p.UpdatedAt = time.Now().UTC()
	evt := Event{
		ID:        "evt_" + evtUUID.String(),
		PaymentID: p.ID,
		Type:      EventCancelled,
		Metadata:  "{}",
		RequestID: requestID,
		CreatedAt: p.UpdatedAt,
	}
	ikey := IdempotencyKey{
		MerchantID:  merchantID,
		Key:         key,
		RequestHash: string(fingerprint),
		PaymentID:   p.ID,
	}
	err = s.store.SaveTransition(ctx, p, from, evt, &ikey, nil)
	if errors.Is(err, errDuplicateIdempotencyKey) || errors.Is(err, errStaleVersion) {
		return s.store.Get(ctx, p.ID)
	}
	return p, err
}
