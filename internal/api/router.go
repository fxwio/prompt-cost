package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func NewRouter(h *Handler, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", h.Health)
	r.Handle("/metrics", promhttp.Handler())

	// Prompt templates
	r.Route("/templates", func(r chi.Router) {
		r.Get("/", h.ListTemplates)
		r.Post("/", h.CreateTemplate)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetTemplate)
			r.Delete("/", h.DeleteTemplate)
			r.Post("/render", h.RenderTemplate)
			r.Route("/versions", func(r chi.Router) {
				r.Get("/", h.ListVersions)
				r.Post("/", h.CreateVersion)
				r.Put("/{version}/activate", h.ActivateVersion)
			})
		})
	})

	// Usage & cost
	r.Route("/usage", func(r chi.Router) {
		r.Post("/", h.RecordUsage)
		r.Get("/", h.ListUsage)
	})
	r.Get("/cost/report", h.CostReport)
	r.Get("/cost/models", h.ListModels)

	return r
}

func requestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			logger.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("latency", time.Since(start)),
			)
		})
	}
}
