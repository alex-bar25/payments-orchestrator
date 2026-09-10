package httpapi

import (
	"net/http"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
)

func (h *Handler) listDiscrepancies(w http.ResponseWriter, r *http.Request) {
	var rows []payment.Discrepancy
	if err := h.db.WithContext(r.Context()).Order("created_at").Find(&rows).Error; err != nil {
		writeErr(w, h.logger, err)
		return
	}
	if rows == nil {
		rows = []payment.Discrepancy{}
	}
	writeJSON(w, http.StatusOK, rows)
}
