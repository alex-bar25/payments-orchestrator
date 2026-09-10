package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
)

type inboundRequest struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	PaymentID string `json:"payment_id"`
}

func (h *Handler) mockWebhook(w http.ResponseWriter, r *http.Request) {
	var req inboundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be JSON with id, type, and payment_id")
		return
	}
	p, err := h.payments.ApplyInbound(r.Context(), payment.InboundInput{
		ID:        req.ID,
		Type:      req.Type,
		PaymentID: req.PaymentID,
	})
	if err != nil {
		writeErr(w, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
