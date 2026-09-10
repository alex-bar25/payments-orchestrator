package payment

import (
	"context"
	"time"
)

func (s *Service) writeLedger(ctx context.Context, paymentID string, providerID *string, status Status) error {
	pid := ""
	if providerID != nil {
		pid = *providerID
	}
	return s.store.UpsertProviderLedger(ctx, ProviderLedger{
		PaymentID:         paymentID,
		ProviderPaymentID: pid,
		Status:            string(status),
		UpdatedAt:         time.Now().UTC(),
	})
}
