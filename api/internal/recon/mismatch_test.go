package recon

import (
	"testing"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
)

func TestFindMismatchesSkipsInFlightAndMatches(t *testing.T) {
	payments := []payment.Payment{
		{ID: "a", Status: payment.StatusRequiresPayment},
		{ID: "b", Status: payment.StatusProcessing},
		{ID: "c", Status: payment.StatusAuthorized},
	}
	ledgers := map[string]payment.ProviderLedger{
		"c": {PaymentID: "c", Status: string(payment.StatusAuthorized)},
	}
	if got := FindMismatches(payments, ledgers); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestFindMismatchesRecordsProviderAhead(t *testing.T) {
	payments := []payment.Payment{{ID: "p", Status: payment.StatusProcessing}}
	ledgers := map[string]payment.ProviderLedger{
		"p": {PaymentID: "p", Status: string(payment.StatusAuthorized)},
	}
	got := FindMismatches(payments, ledgers)
	if len(got) != 1 || got[0].Internal != string(payment.StatusProcessing) || got[0].Provider != string(payment.StatusAuthorized) {
		t.Fatalf("got %+v", got)
	}
}

func TestFindMismatchesCancelVsAuthorized(t *testing.T) {
	payments := []payment.Payment{{ID: "p", Status: payment.StatusCancelled}}
	ledgers := map[string]payment.ProviderLedger{
		"p": {PaymentID: "p", Status: string(payment.StatusAuthorized)},
	}
	got := FindMismatches(payments, ledgers)
	if len(got) != 1 || got[0].Provider != string(payment.StatusAuthorized) {
		t.Fatalf("got %+v", got)
	}
}
