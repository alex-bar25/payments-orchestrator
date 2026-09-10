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
