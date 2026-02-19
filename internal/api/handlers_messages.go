package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shohag/piperelay/internal/models"
	"github.com/shohag/piperelay/internal/storage"
)

type MessageHandler struct {
	store storage.Storage
}

func NewMessageHandler(store storage.Storage) *MessageHandler {
	return &MessageHandler{store: store}
}

type sendMessageRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

const maxPayloadSize = 256 * 1024 // 256KB

func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	app := AppFromContext(r.Context())
	if app == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSize)
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.EventType == "" {
		writeError(w, http.StatusBadRequest, "event_type is required")
		return
	}
	if len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}

	now := time.Now().UTC()
	msg := &models.Message{
		ID:        models.NewID("msg"),
		AppID:     app.ID,
		EventType: req.EventType,
		Payload:   req.Payload,
		CreatedAt: now,
	}

	if err := h.store.CreateMessage(r.Context(), msg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create message")
		return
	}

	// Find matching endpoints and create deliveries
	endpoints, err := h.store.GetEndpointsByEventType(r.Context(), app.ID, req.EventType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find endpoints")
		return
	}

	deliveries := make([]models.Delivery, 0, len(endpoints))
	for _, ep := range endpoints {
		d := models.Delivery{
			ID:         models.NewID("dlv"),
			MessageID:  msg.ID,
			EndpointID: ep.ID,
			Status:     models.DeliveryPending,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := h.store.CreateDelivery(r.Context(), &d); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create delivery")
			return
		}
		deliveries = append(deliveries, d)
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":    msg,
		"deliveries": len(deliveries),
	})
}

func (h *MessageHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	msg, err := h.store.GetMessage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get message")
		return
	}
	if msg == nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	deliveries, err := h.store.GetDeliveriesByMessage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get deliveries")
		return
	}
	if deliveries == nil {
		deliveries = []models.Delivery{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    msg,
		"deliveries": deliveries,
	})
}

func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	app := AppFromContext(r.Context())
	if app == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	msgs, err := h.store.ListMessages(r.Context(), app.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}
	if msgs == nil {
		msgs = []models.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (h *MessageHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	msg, err := h.store.GetMessage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get message")
		return
	}
	if msg == nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	deliveries, err := h.store.GetDeliveriesByMessage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get deliveries")
		return
	}

	retried := 0
	now := time.Now().UTC()
	for _, d := range deliveries {
		if d.Status == models.DeliveryFailed {
			d.Status = models.DeliveryRetrying
			d.NextRetryAt = &now
			if err := h.store.UpdateDelivery(r.Context(), &d); err != nil {
				continue
			}
			retried++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"retried": retried,
	})
}
