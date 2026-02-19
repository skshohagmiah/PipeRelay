package api

import (
	"net/http"

	"github.com/shohag/piperelay/internal/storage"
)

type StatsHandler struct {
	store storage.Storage
}

func NewStatsHandler(store storage.Storage) *StatsHandler {
	return &StatsHandler{store: store}
}

func (h *StatsHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "piperelay",
	})
}

func (h *StatsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	app := AppFromContext(r.Context())
	if app == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	stats, err := h.store.GetStats(r.Context(), app.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
