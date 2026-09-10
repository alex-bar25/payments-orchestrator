package payment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) Authorize(ctx context.Context, merchantID, paymentID, idempotencyKey, requestID string) (Payment, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 255 {
		return Payment{}, ErrInvalidIdempotencyKey
	}
	fingerprint, err := json.Marshal(struct {
		Op        string `json:"op"`
		PaymentID string `json:"payment_id"`
	}{"authorize", paymentID})
	if err != nil {
		return Payment{}, err
	}

	rec, err := s.store.LookupIdempotency(ctx, merchantID, key)
	if err == nil {
		return s.resumeAuthorize(ctx, rec, string(fingerprint), requestID)
	}
	if !errors.Is(err, ErrNotFound) {
		return Payment{}, err
	}

	p, err := s.store.Get(ctx, paymentID)
	if err != nil {
		return Payment{}, err
	}
	if p.Status == StatusProcessing {
		return s.completeAuthorize(ctx, p, requestID)
	}
	if p.Status != StatusRequiresPayment {
		return Payment{}, ErrInvalidPaymentState
	}

	evtUUID, err := uuid.NewRandom()
	if err != nil {
		return Payment{}, err
	}
	from := p.Status
	p.Status = StatusProcessing
	p.Version++
	p.UpdatedAt = time.Now().UTC()
	evt := Event{
		ID:        "evt_" + evtUUID.String(),
		PaymentID: p.ID,
		Type:      EventProcessing,
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
	if errors.Is(err, errDuplicateIdempotencyKey) {
		rec, err := s.store.LookupIdempotency(ctx, merchantID, key)
		if err != nil {
			return Payment{}, err
		}
		return s.resumeAuthorize(ctx, rec, string(fingerprint), requestID)
	}
	if err != nil {
		return Payment{}, err
	}
	return s.completeAuthorize(ctx, p, requestID)
}

func (s *Service) resumeAuthorize(ctx context.Context, rec IdempotencyKey, fingerprint, requestID string) (Payment, error) {
	if rec.RequestHash != fingerprint {
		return Payment{}, ErrIdempotencyKeyReuse
	}
	p, err := s.store.Get(ctx, rec.PaymentID)
	if err != nil {
		return Payment{}, err
	}
	if p.Status == StatusAuthorized || p.Status == StatusFailed {
		return p, nil
	}
	return s.completeAuthorize(ctx, p, requestID)
}

func (s *Service) completeAuthorize(ctx context.Context, p Payment, requestID string) (Payment, error) {
	if p.Status == StatusAuthorized || p.Status == StatusFailed {
		return p, nil
	}
	if p.Status != StatusProcessing {
		return Payment{}, ErrInvalidPaymentState
	}

	res, err := s.psp.Authorize(ctx, p.Amount, p.Currency)
	if err != nil {
		return Payment{}, err
	}
	typ := EventFailed
	p.Status = StatusFailed
	if res.OK {
		typ = EventAuthorized
		p.Status = StatusAuthorized
		p.ProviderPaymentID = &res.ProviderID
	}
	if err := s.writeLedger(ctx, p.ID, p.ProviderPaymentID, p.Status); err != nil {
		return Payment{}, err
	}

	evtUUID, err := uuid.NewRandom()
	if err != nil {
		return Payment{}, err
	}
	from := StatusProcessing
	p.Version++
	p.UpdatedAt = time.Now().UTC()
	evt := Event{
		ID:        "evt_" + evtUUID.String(),
		PaymentID: p.ID,
		Type:      typ,
		Metadata:  "{}",
		RequestID: requestID,
		CreatedAt: p.UpdatedAt,
	}
	err = s.store.SaveTransition(ctx, p, from, evt, nil, nil)
	if errors.Is(err, errStaleVersion) {
		return s.store.Get(ctx, p.ID)
	}
	return p, err
}
