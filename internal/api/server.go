package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/shohag/piperelay/internal/config"
	"github.com/shohag/piperelay/internal/storage"
)

type Server struct {
	cfg    config.ServerConfig
	store  storage.Storage
	router *chi.Mux
	log    zerolog.Logger
	http   *http.Server
}

func NewServer(cfg config.ServerConfig, store storage.Storage, log zerolog.Logger) *Server {
	s := &Server{
		cfg:   cfg,
		store: store,
		log:   log,
	}
	s.router = s.buildRouter()
	return s
}

func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(LoggingMiddleware(s.log))

	appHandler := NewApplicationHandler(s.store)
	epHandler := NewEndpointHandler(s.store)
	msgHandler := NewMessageHandler(s.store)
	dlvHandler := NewDeliveryHandler(s.store)
	statsHandler := NewStatsHandler(s.store)

	// Health check — no auth
	r.Get("/health", statsHandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		// Application management — no bearer auth (admin routes)
		r.Post("/applications", appHandler.Create)
		r.Get("/applications", appHandler.List)
		r.Get("/applications/{id}", appHandler.Get)
		r.Delete("/applications/{id}", appHandler.Delete)
		r.Post("/applications/{id}/rotate-key", appHandler.RotateKey)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(s.store))

			// Endpoints
			r.Post("/endpoints", epHandler.Create)
			r.Get("/endpoints", epHandler.List)
			r.Get("/endpoints/{id}", epHandler.Get)
			r.Put("/endpoints/{id}", epHandler.Update)
			r.Delete("/endpoints/{id}", epHandler.Delete)
			r.Patch("/endpoints/{id}/toggle", epHandler.Toggle)
			r.Get("/endpoints/{id}/stats", epHandler.Stats)

			// Messages
			r.Post("/messages", msgHandler.Send)
			r.Get("/messages", msgHandler.List)
			r.Get("/messages/{id}", msgHandler.Get)
			r.Post("/messages/{id}/retry", msgHandler.Retry)

			// Deliveries
			r.Get("/deliveries/{id}", dlvHandler.Get)
			r.Get("/deliveries/{id}/attempts", dlvHandler.ListAttempts)

			// Stats
			r.Get("/stats", statsHandler.Stats)
		})
	})

	return r
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.http = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
	}

	s.log.Info().Str("addr", addr).Msg("starting HTTP server")
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.http.Shutdown(ctx)
}
