package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shohag/piperelay/internal/models"
	"github.com/shohag/piperelay/internal/storage"
)

type ApplicationHandler struct {
	store storage.Storage
}

func NewApplicationHandler(store storage.Storage) *ApplicationHandler {
	return &ApplicationHandler{store: store}
}

type createAppRequest struct {
	Name string `json:"name"`
}

func (h *ApplicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	now := time.Now().UTC()
	app := &models.Application{
		ID:        models.NewID("app"),
		Name:      req.Name,
		APIKey:    models.NewAPIKey(),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.CreateApplication(r.Context(), app); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create application")
		return
	}

	writeJSON(w, http.StatusCreated, app)
}

func (h *ApplicationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := h.store.GetApplication(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get application")
		return
	}
	if app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	app.APIKey = "" // don't expose
	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) List(w http.ResponseWriter, r *http.Request) {
	apps, err := h.store.ListApplications(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}
	for i := range apps {
		apps[i].APIKey = "" // don't expose
	}
	if apps == nil {
		apps = []models.Application{}
	}
	writeJSON(w, http.StatusOK, apps)
}

func (h *ApplicationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := h.store.GetApplication(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get application")
		return
	}
	if app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	if err := h.store.DeleteApplication(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete application")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ApplicationHandler) RotateKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := h.store.GetApplication(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get application")
		return
	}
	if app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	newKey := models.NewAPIKey()
	if err := h.store.UpdateApplicationAPIKey(r.Context(), id, newKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"api_key": newKey})
}
