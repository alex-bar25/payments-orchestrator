package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
)

func writeErr(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, payment.ErrInvalidAmount):
		writeAPIError(w, http.StatusBadRequest, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, payment.ErrInvalidCurrency):
		writeAPIError(w, http.StatusBadRequest, "INVALID_CURRENCY", err.Error())
	case errors.Is(err, payment.ErrInvalidIdempotencyKey):
		writeAPIError(w, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", err.Error())
	case errors.Is(err, payment.ErrIdempotencyKeyReuse):
		writeAPIError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", err.Error())
	case errors.Is(err, payment.ErrInvalidPaymentState):
		writeAPIError(w, http.StatusConflict, "INVALID_PAYMENT_STATE", err.Error())
	case errors.Is(err, payment.ErrCaptureDeclined):
		writeAPIError(w, http.StatusConflict, "CAPTURE_DECLINED", err.Error())
	case errors.Is(err, payment.ErrRefundDeclined):
		writeAPIError(w, http.StatusConflict, "REFUND_DECLINED", err.Error())
	case errors.Is(err, payment.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "PAYMENT_NOT_FOUND", err.Error())
	default:
		logger.Error("request failed", "err", err)
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
