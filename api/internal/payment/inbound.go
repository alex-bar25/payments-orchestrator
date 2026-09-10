package payment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type InboundInput struct {
	ID        string
	Type      string
	PaymentID string
}

func inboundTransition(typ string) (from, to Status, ok bool) {
	switch typ {
	case EventAuthorized:
		return StatusProcessing, StatusAuthorized, true
	case EventFailed:
		return StatusProcessing, StatusFailed, true
	case EventCaptured:
		return StatusAuthorized, StatusCaptured, true
	case EventRefunded:
		return StatusCaptured, StatusRefunded, true
	case EventCancelled:
		return StatusAuthorized, StatusCancelled, true
	default:
		return "", "", false
	}
}

func (s *Service) ApplyInbound(ctx context.Context, in InboundInput) (Payment, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" || len(id) > 255 {
		return Payment{}, ErrInvalidProviderEvent
	}
	from, to, ok := inboundTransition(in.Type)
	if !ok {
		return Payment{}, ErrInvalidProviderEvent
	}
	fingerprint, err := json.Marshal(struct {
		Type      string `json:"type"`
		PaymentID string `json:"payment_id"`
	}{in.Type, in.PaymentID})
	if err != nil {
		return Payment{}, err
	}

	rec, err := s.store.LookupProviderEvent(ctx, id)
	if err == nil {
		if rec.RequestHash != string(fingerprint) {
			return Payment{}, ErrProviderEventReuse
		}
		return s.store.Get(ctx, rec.PaymentID)
	}
	if !errors.Is(err, ErrNotFound) {
		return Payment{}, err
	}

	p, err := s.store.Get(ctx, in.PaymentID)
	if err != nil {
		return Payment{}, err
	}
	pe := ProviderEvent{
		ID:          id,
		RequestHash: string(fingerprint),
		PaymentID:   p.ID,
		CreatedAt:   time.Now().UTC(),
	}
	if p.Status == to {
		err = s.store.InsertProviderEvent(ctx, pe)
		if errors.Is(err, errDuplicateProviderEvent) {
			rec, err := s.store.LookupProviderEvent(ctx, id)
			if err != nil {
				return Payment{}, err
			}
			if rec.RequestHash != string(fingerprint) {
				return Payment{}, ErrProviderEventReuse
			}
			return s.store.Get(ctx, rec.PaymentID)
		}
		return p, err
	}
	if p.Status != from {
		return Payment{}, ErrInvalidPaymentState
	}
	if err := s.writeLedger(ctx, p.ID, p.ProviderPaymentID, to); err != nil {
		return Payment{}, err
	}

	evtUUID, err := uuid.NewRandom()
	if err != nil {
		return Payment{}, err
	}
	now := time.Now().UTC()
	p.Status = to
	p.Version++
	p.UpdatedAt = now
	evt := Event{
		ID:        "evt_" + evtUUID.String(),
		PaymentID: p.ID,
		Type:      in.Type,
		Metadata:  "{}",
		CreatedAt: now,
	}
	err = s.store.SaveTransition(ctx, p, from, evt, nil, &pe)
	if errors.Is(err, errDuplicateProviderEvent) {
		rec, err := s.store.LookupProviderEvent(ctx, id)
		if err != nil {
			return Payment{}, err
		}
		if rec.RequestHash != string(fingerprint) {
			return Payment{}, ErrProviderEventReuse
		}
		return s.store.Get(ctx, rec.PaymentID)
	}
	return p, err
}
