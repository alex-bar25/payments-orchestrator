package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"
)

type Handler struct {
	logger   *slog.Logger
	db       *gorm.DB
	payments *payment.Service
	store    payment.Store
}

func New(logger *slog.Logger, gdb *gorm.DB, payments *payment.Service, store payment.Store) http.Handler {
	h := &Handler{logger: logger, db: gdb, payments: payments, store: store}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/payments", h.createPayment)
		r.Get("/payments/{id}", h.getPayment)
		r.Post("/payments/{id}/authorize", h.authorizePayment)
	})
	return r
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := h.db.DB()
	if err != nil {
		h.logger.Warn("readiness check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "dependency": "postgres"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		h.logger.Warn("readiness check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "dependency": "postgres"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
