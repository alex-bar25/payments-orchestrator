package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
	"github.com/go-chi/chi/v5"
)

type createRequest struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func (h *Handler) createPayment(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be JSON with amount and currency")
		return
	}

	p, err := h.payments.Create(r.Context(), payment.CreateInput{
		MerchantID:     payment.DemoMerchantID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		RequestID:      r.Header.Get("X-Request-Id"),
	})
	if err != nil {
		writeErr(w, h.logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
