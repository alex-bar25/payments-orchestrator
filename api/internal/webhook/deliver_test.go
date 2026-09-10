package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
)

func TestDeliverSucceedsOn2xx(t *testing.T) {
	ev := payment.WebhookEvent{ID: "wh_1", Payload: `{"id":"wh_1"}`}
	secret := "s3cret"
	var gotID, gotSig, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-Webhook-Id")
		gotSig = r.Header.Get("X-Webhook-Signature")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := Deliver(context.Background(), srv.Client(), srv.URL, secret, ev); err != nil {
		t.Fatal(err)
	}
	if gotID != ev.ID || gotBody != ev.Payload {
		t.Fatalf("id=%s body=%s", gotID, gotBody)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ev.Payload))
	if gotSig != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatalf("signature mismatch")
	}
}

func TestDeliverFailsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	err := Deliver(context.Background(), srv.Client(), srv.URL, "", payment.WebhookEvent{ID: "wh_1", Payload: "{}"})
	if err == nil {
		t.Fatal("expected error")
	}
}
