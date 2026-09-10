package payment

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type memStore struct {
	payments map[string]Payment
	events   []Event
	keys     map[string]IdempotencyKey
}

func newMemStore() *memStore {
	return &memStore{
		payments: map[string]Payment{},
		keys:     map[string]IdempotencyKey{},
	}
}

func (m *memStore) InsertCreated(_ context.Context, p Payment, e Event, key IdempotencyKey) error {
	k := key.MerchantID + "\x00" + key.Key
	if _, ok := m.keys[k]; ok {
		return errDuplicateIdempotencyKey
	}
	m.payments[p.ID] = p
	m.events = append(m.events, e)
	m.keys[k] = key
	return nil
}

func (m *memStore) LookupIdempotency(_ context.Context, merchantID, key string) (IdempotencyKey, error) {
	rec, ok := m.keys[merchantID+"\x00"+key]
	if !ok {
		return IdempotencyKey{}, ErrNotFound
	}
	return rec, nil
}

func (m *memStore) Get(_ context.Context, id string) (Payment, error) {
	p, ok := m.payments[id]
	if !ok {
		return Payment{}, ErrNotFound
	}
	return p, nil
}

func (m *memStore) SaveTransition(_ context.Context, p Payment, from Status, e Event, key *IdempotencyKey) error {
	cur, ok := m.payments[p.ID]
	if !ok {
		return ErrNotFound
	}
	if cur.Status != from || cur.Version != p.Version-1 {
		return errStaleVersion
	}
	if key != nil {
		k := key.MerchantID + "\x00" + key.Key
		if _, exists := m.keys[k]; exists {
			return errDuplicateIdempotencyKey
		}
		m.keys[k] = *key
	}
	m.payments[p.ID] = p
	m.events = append(m.events, e)
	return nil
}

func validCreateInput() CreateInput {
	return CreateInput{
		MerchantID:     DemoMerchantID,
		Amount:         4999,
		Currency:       "EUR",
		IdempotencyKey: "order_123",
	}
}

func TestCreateReplayDoesNotDuplicate(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	first, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Fatalf("replay created %s, want %s", second.ID, first.ID)
	}
	if len(store.events) != 1 {
		t.Fatalf("events = %d, want 1", len(store.events))
	}
}

func TestCreateReplayTreatsCurrencyCaseAsSameRequest(t *testing.T) {
	svc := NewService(newMemStore())
	in := validCreateInput()
	in.Currency = "eur"
	first, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.Currency = "EUR"
	second, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("eur and EUR with the same key must replay, not create")
	}
}

func TestCreateReplayTrimsIdempotencyKey(t *testing.T) {
	svc := NewService(newMemStore())
	in := validCreateInput()
	first, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.IdempotencyKey = "  order_123  "
	second, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("padded idempotency key must replay")
	}
}

func TestCreateRejectsKeyReuseWhenBodyChanges(t *testing.T) {
	svc := NewService(newMemStore())
	if _, err := svc.Create(context.Background(), validCreateInput()); err != nil {
		t.Fatal(err)
	}
	in := validCreateInput()
	in.Amount = 1000
	_, err := svc.Create(context.Background(), in)
	if !errors.Is(err, ErrIdempotencyKeyReuse) {
		t.Fatalf("err = %v, want %v", err, ErrIdempotencyKeyReuse)
	}
}

func TestCreateRejectsZeroAndNegativeAmount(t *testing.T) {
	svc := NewService(newMemStore())
	for _, amount := range []int64{0, -1} {
		in := validCreateInput()
		in.Amount = amount
		_, err := svc.Create(context.Background(), in)
		if !errors.Is(err, ErrInvalidAmount) {
			t.Fatalf("amount %d: err = %v, want %v", amount, err, ErrInvalidAmount)
		}
	}
}

func TestCreateIdempotencyKeyLengthBoundary(t *testing.T) {
	svc := NewService(newMemStore())
	in := validCreateInput()
	in.IdempotencyKey = strings.Repeat("k", 255)
	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("255-char key should be accepted: %v", err)
	}
	in.IdempotencyKey = strings.Repeat("k", 256)
	_, err := svc.Create(context.Background(), in)
	if !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("256-char key: err = %v, want %v", err, ErrInvalidIdempotencyKey)
	}
}

func TestAuthorizeSucceeds(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusAuthorized {
		t.Fatalf("status = %s", got.Status)
	}
	if got.ProviderPaymentID == nil {
		t.Fatal("missing provider payment id")
	}
	if len(store.events) != 3 {
		t.Fatalf("events = %d, want created+processing+authorized", len(store.events))
	}
}

func TestAuthorizeDeclinesOneCent(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	in := validCreateInput()
	in.Amount = 1
	in.IdempotencyKey = "order_decline"
	p, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_decline", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %s", got.Status)
	}
	if got.ProviderPaymentID != nil {
		t.Fatal("declined payment should not have a provider id")
	}
}

func TestAuthorizeRejectsCaptured(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	cur := store.payments[p.ID]
	cur.Status = StatusCaptured
	store.payments[p.ID] = cur
	_, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_captured", "")
	if !errors.Is(err, ErrInvalidPaymentState) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidPaymentState)
	}
}

func TestAuthorizeReplayDoesNotReauth(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || second.Status != StatusAuthorized {
		t.Fatalf("replay status = %s id = %s", second.Status, second.ID)
	}
	if len(store.events) != 3 {
		t.Fatalf("replay emitted extra events: %d", len(store.events))
	}
}

func TestAuthorizeRejectsCreateIdempotencyKey(t *testing.T) {
	svc := NewService(newMemStore())
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "order_123", "")
	if !errors.Is(err, ErrIdempotencyKeyReuse) {
		t.Fatalf("err = %v, want %v", err, ErrIdempotencyKeyReuse)
	}
}

func TestCaptureSucceeds(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCaptured {
		t.Fatalf("status = %s", got.Status)
	}
	if len(store.events) != 4 {
		t.Fatalf("events = %d, want 4", len(store.events))
	}
}

func TestCaptureRejectsUnauthorized(t *testing.T) {
	svc := NewService(newMemStore())
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_too_soon", "")
	if !errors.Is(err, ErrInvalidPaymentState) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidPaymentState)
	}
}

func TestCaptureReplayDoesNotRecapture(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_1", ""); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCaptured {
		t.Fatalf("status = %s", got.Status)
	}
	if len(store.events) != 4 {
		t.Fatalf("replay emitted extra events: %d", len(store.events))
	}
}

func TestCaptureDeclineLeavesAuthorized(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	in := validCreateInput()
	in.Amount = 2
	in.IdempotencyKey = "order_cap_fail"
	p, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_cap_fail", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_fail", "")
	if !errors.Is(err, ErrCaptureDeclined) {
		t.Fatalf("err = %v, want %v", err, ErrCaptureDeclined)
	}
	got, err := store.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusAuthorized {
		t.Fatalf("status = %s, want authorized", got.Status)
	}
}

func TestRefundSucceeds(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_1", "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Refund(context.Background(), DemoMerchantID, p.ID, "ref_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRefunded {
		t.Fatalf("status = %s", got.Status)
	}
	if len(store.events) != 5 {
		t.Fatalf("events = %d, want 5", len(store.events))
	}
}

func TestRefundRejectsUncaptured(t *testing.T) {
	svc := NewService(newMemStore())
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Refund(context.Background(), DemoMerchantID, p.ID, "ref_too_soon", "")
	if !errors.Is(err, ErrInvalidPaymentState) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidPaymentState)
	}
}

func TestRefundReplayDoesNotRerefund(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	p, err := svc.Create(context.Background(), validCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_1", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(context.Background(), DemoMerchantID, p.ID, "ref_1", ""); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Refund(context.Background(), DemoMerchantID, p.ID, "ref_1", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRefunded {
		t.Fatalf("status = %s", got.Status)
	}
	if len(store.events) != 5 {
		t.Fatalf("replay emitted extra events: %d", len(store.events))
	}
}

func TestRefundDeclineLeavesCaptured(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)
	in := validCreateInput()
	in.Amount = 3
	in.IdempotencyKey = "order_ref_fail"
	p, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Authorize(context.Background(), DemoMerchantID, p.ID, "auth_ref_fail", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.Capture(context.Background(), DemoMerchantID, p.ID, "cap_ref_fail", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Refund(context.Background(), DemoMerchantID, p.ID, "ref_fail", "")
	if !errors.Is(err, ErrRefundDeclined) {
		t.Fatalf("err = %v, want %v", err, ErrRefundDeclined)
	}
	got, err := store.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCaptured {
		t.Fatalf("status = %s, want captured", got.Status)
	}
}
