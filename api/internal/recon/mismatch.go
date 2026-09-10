package recon

import "github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"

type Mismatch struct {
	PaymentID string
	Internal  string
	Provider  string
}

func FindMismatches(payments []payment.Payment, ledgers map[string]payment.ProviderLedger) []Mismatch {
	var out []Mismatch
	for _, p := range payments {
		if p.Status == payment.StatusRequiresPayment {
			continue
		}
		led, ok := ledgers[p.ID]
		if p.Status == payment.StatusProcessing && !ok {
			continue
		}
		if !ok {
			out = append(out, Mismatch{PaymentID: p.ID, Internal: string(p.Status), Provider: "missing"})
			continue
		}
		if led.Status == string(p.Status) {
			continue
		}
		out = append(out, Mismatch{PaymentID: p.ID, Internal: string(p.Status), Provider: led.Status})
	}
	return out
}
