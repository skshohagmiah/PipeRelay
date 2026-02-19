package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/shohag/piperelay/internal/models"
	"github.com/shohag/piperelay/internal/storage"
)

type contextKey string

const appContextKey contextKey = "application"

func AppFromContext(ctx context.Context) *models.Application {
	app, _ := ctx.Value(appContextKey).(*models.Application)
	return app
}

func AuthMiddleware(store storage.Storage) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			apiKey := strings.TrimPrefix(auth, "Bearer ")
			if apiKey == auth {
				writeError(w, http.StatusUnauthorized, "invalid authorization format, use: Bearer <api_key>")
				return
			}

			app, err := store.GetApplicationByAPIKey(r.Context(), apiKey)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if app == nil {
				writeError(w, http.StatusUnauthorized, "invalid api key")
				return
			}

			ctx := context.WithValue(r.Context(), appContextKey, app)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func LoggingMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.statusCode).
				Dur("duration", time.Since(start)).
				Msg("request")
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
