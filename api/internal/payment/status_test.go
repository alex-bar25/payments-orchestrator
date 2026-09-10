package payment

import "testing"

func TestCanTransition(t *testing.T) {
	all := []Status{
		StatusRequiresPayment,
		StatusProcessing,
		StatusAuthorized,
		StatusCaptured,
		StatusRefunded,
		StatusFailed,
		StatusCancelled,
	}

	allowed := map[[2]Status]struct{}{
		{StatusRequiresPayment, StatusProcessing}: {},
		{StatusProcessing, StatusAuthorized}:      {},
		{StatusProcessing, StatusFailed}:          {},
		{StatusAuthorized, StatusCaptured}:        {},
		{StatusAuthorized, StatusCancelled}:       {},
		{StatusCaptured, StatusRefunded}:          {},
	}

	for _, from := range all {
		for _, to := range all {
			_, want := allowed[[2]Status{from, to}]
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestCanTransitionUnknownStatus(t *testing.T) {
	if CanTransition(Status("nope"), StatusProcessing) {
		t.Fatal("unknown status should not transition")
	}
}
