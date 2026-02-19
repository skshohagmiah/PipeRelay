package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shohag/piperelay/internal/models"
	"github.com/shohag/piperelay/internal/storage"
)

type DeliveryHandler struct {
	store storage.Storage
}

func NewDeliveryHandler(store storage.Storage) *DeliveryHandler {
	return &DeliveryHandler{store: store}
}

func (h *DeliveryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.store.GetDelivery(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get delivery")
		return
	}
	if d == nil {
		writeError(w, http.StatusNotFound, "delivery not found")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *DeliveryHandler) ListAttempts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	attempts, err := h.store.GetAttemptsByDelivery(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get attempts")
		return
	}
	if attempts == nil {
		attempts = []models.Attempt{}
	}
	writeJSON(w, http.StatusOK, attempts)
}
