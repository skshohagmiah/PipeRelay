package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shohag/piperelay/internal/models"
	"github.com/shohag/piperelay/internal/storage"
)

type EndpointHandler struct {
	store storage.Storage
}

func NewEndpointHandler(store storage.Storage) *EndpointHandler {
	return &EndpointHandler{store: store}
}

type createEndpointRequest struct {
	URL         string            `json:"url"`
	Description string            `json:"description"`
	EventTypes  []string          `json:"event_types"`
	RateLimit   int               `json:"rate_limit"`
	Metadata    map[string]string `json:"metadata"`
}

func (h *EndpointHandler) Create(w http.ResponseWriter, r *http.Request) {
	app := AppFromContext(r.Context())
	if app == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "url must be a valid HTTP or HTTPS URL")
		return
	}

	now := time.Now().UTC()
	ep := &models.Endpoint{
		ID:          models.NewID("ep"),
		AppID:       app.ID,
		URL:         req.URL,
		Description: req.Description,
		Secret:      models.NewSecret(),
		EventTypes:  req.EventTypes,
		RateLimit:   req.RateLimit,
		Metadata:    req.Metadata,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if ep.EventTypes == nil {
		ep.EventTypes = []string{}
	}
	if ep.Metadata == nil {
		ep.Metadata = map[string]string{}
	}

	if err := h.store.CreateEndpoint(r.Context(), ep); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create endpoint")
		return
	}

	writeJSON(w, http.StatusCreated, ep)
}

func (h *EndpointHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ep, err := h.store.GetEndpoint(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get endpoint")
		return
	}
	if ep == nil {
		writeError(w, http.StatusNotFound, "endpoint not found")
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (h *EndpointHandler) List(w http.ResponseWriter, r *http.Request) {
	app := AppFromContext(r.Context())
	if app == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	eps, err := h.store.ListEndpoints(r.Context(), app.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}
	if eps == nil {
		eps = []models.Endpoint{}
	}
	writeJSON(w, http.StatusOK, eps)
}

type updateEndpointRequest struct {
	URL         string            `json:"url"`
	Description string            `json:"description"`
	EventTypes  []string          `json:"event_types"`
	RateLimit   int               `json:"rate_limit"`
	Metadata    map[string]string `json:"metadata"`
}

func (h *EndpointHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ep, err := h.store.GetEndpoint(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get endpoint")
		return
	}
	if ep == nil {
		writeError(w, http.StatusNotFound, "endpoint not found")
		return
	}

	var req updateEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL != "" {
		u, err := url.Parse(req.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			writeError(w, http.StatusBadRequest, "url must be a valid HTTP or HTTPS URL")
			return
		}
		ep.URL = req.URL
	}
	ep.Description = req.Description
	if req.EventTypes != nil {
		ep.EventTypes = req.EventTypes
	}
	ep.RateLimit = req.RateLimit
	if req.Metadata != nil {
		ep.Metadata = req.Metadata
	}

	if err := h.store.UpdateEndpoint(r.Context(), ep); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update endpoint")
		return
	}

	writeJSON(w, http.StatusOK, ep)
}

func (h *EndpointHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ep, err := h.store.GetEndpoint(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get endpoint")
		return
	}
	if ep == nil {
		writeError(w, http.StatusNotFound, "endpoint not found")
		return
	}

	if err := h.store.DeleteEndpoint(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete endpoint")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EndpointHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ep, err := h.store.GetEndpoint(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get endpoint")
		return
	}
	if ep == nil {
		writeError(w, http.StatusNotFound, "endpoint not found")
		return
	}

	newActive := !ep.Active
	if err := h.store.ToggleEndpoint(r.Context(), id, newActive); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle endpoint")
		return
	}

	ep.Active = newActive
	writeJSON(w, http.StatusOK, ep)
}

func (h *EndpointHandler) Stats(w http.ResponseWriter, r *http.Request) {
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
