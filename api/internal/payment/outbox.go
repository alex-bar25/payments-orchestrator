package payment

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	WebhookPending   = "pending"
	WebhookDelivered = "delivered"
	WebhookFailed    = "failed"
)

type WebhookEvent struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	PaymentID     string    `json:"payment_id"`
	Type          string    `json:"type"`
	Payload       string    `json:"payload" gorm:"type:jsonb"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	LastError     string    `json:"last_error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (WebhookEvent) TableName() string { return "webhook_events" }

func newOutboxEvent(p Payment, e Event) (WebhookEvent, bool, error) {
	if e.Type == EventProcessing {
		return WebhookEvent{}, false, nil
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return WebhookEvent{}, false, err
	}
	wid := "wh_" + id.String()
	body, err := json.Marshal(struct {
		ID        string    `json:"id"`
		Type      string    `json:"type"`
		CreatedAt time.Time `json:"created_at"`
		Payment   Payment   `json:"payment"`
	}{wid, e.Type, e.CreatedAt, p})
	if err != nil {
		return WebhookEvent{}, false, err
	}
	now := e.CreatedAt
	return WebhookEvent{
		ID:            wid,
		PaymentID:     p.ID,
		Type:          e.Type,
		Payload:       string(body),
		Status:        WebhookPending,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, true, nil
}
