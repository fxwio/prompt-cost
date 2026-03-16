// Package api provides HTTP handlers for the Prompt & Cost Platform.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/fxwio/prompt-cost/internal/metrics"
	"github.com/fxwio/prompt-cost/internal/model"
	"github.com/fxwio/prompt-cost/internal/pricing"
	"github.com/fxwio/prompt-cost/internal/store"
	tmpl "github.com/fxwio/prompt-cost/internal/template"
	"github.com/fxwio/prompt-cost/pkg/pgstore"
	"github.com/go-chi/chi/v5"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	pricing *pricing.Table
	logger  *zap.Logger
}

// New creates a Handler using the given pricing table.
func New(pricingTable *pricing.Table, logger *zap.Logger) *Handler {
	return &Handler{pricing: pricingTable, logger: logger}
}

// ── Utility ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// ── Health ────────────────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"database": pgstore.Available(),
	})
}

// ── Templates ─────────────────────────────────────────────────────────────────

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := store.ListTemplates(r.Context())
	if err != nil {
		h.logger.Error("list templates", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if templates == nil {
		templates = []model.Template{}
	}
	writeJSON(w, http.StatusOK, templates)
}

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	t, err := store.CreateTemplate(r.Context(), &model.Template{
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
	})
	if err != nil {
		h.logger.Error("create template", zap.Error(err))
		if strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "template name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	metrics.TemplatesTotal.Inc()
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := store.GetTemplate(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := store.DeleteTemplate(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Template Versions ─────────────────────────────────────────────────────────

func (h *Handler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Content string `json:"content"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	// Verify template exists
	if _, err := store.GetTemplate(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	variables := tmpl.ExtractVariables(req.Content)
	v, err := store.CreateVersion(r.Context(), id, req.Content, variables)
	if err != nil {
		h.logger.Error("create version", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	metrics.TemplateVersionsTotal.Inc()
	writeJSON(w, http.StatusCreated, v)
}

func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	versions, err := store.ListVersions(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if versions == nil {
		versions = []model.TemplateVersion{}
	}
	writeJSON(w, http.StatusOK, versions)
}

func (h *Handler) ActivateVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	versionStr := chi.URLParam(r, "version")
	version, err := strconv.Atoi(versionStr)
	if err != nil || version < 1 {
		writeError(w, http.StatusBadRequest, "invalid version number")
		return
	}
	if err := store.ActivateVersion(r.Context(), id, version); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	t, _ := store.GetTemplate(r.Context(), id)
	writeJSON(w, http.StatusOK, t)
}

// RenderTemplate renders the active (or specified) version with provided variables.
func (h *Handler) RenderTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.RenderRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Variables == nil {
		req.Variables = map[string]string{}
	}

	t, err := store.GetTemplate(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	version := req.Version
	if version == 0 {
		version = t.ActiveVersion
	}
	if version == 0 {
		writeError(w, http.StatusUnprocessableEntity, "template has no active version")
		return
	}

	v, err := store.GetVersion(r.Context(), id, version)
	if err != nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	rendered, err := tmpl.Render(v.Content, req.Variables)
	if err != nil {
		metrics.RenderRequestsTotal.WithLabelValues("error").Inc()
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	metrics.RenderRequestsTotal.WithLabelValues("ok").Inc()
	writeJSON(w, http.StatusOK, model.RenderResponse{
		Rendered:  rendered,
		Version:   version,
		Variables: v.Variables,
	})
}

// ── Usage Events ──────────────────────────────────────────────────────────────

func (h *Handler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tenant           string            `json:"tenant"`
		App              string            `json:"app"`
		Model            string            `json:"model"`
		PromptTokens     int               `json:"prompt_tokens"`
		CompletionTokens int               `json:"completion_tokens"`
		Metadata         map[string]string `json:"metadata"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	if req.PromptTokens < 0 || req.CompletionTokens < 0 {
		writeError(w, http.StatusBadRequest, "token counts must be non-negative")
		return
	}

	// Calculate cost; 0 if model is unknown (still record the event)
	costUSD, _ := h.pricing.Calculate(req.Model, req.PromptTokens, req.CompletionTokens)

	event := &model.UsageEvent{
		Tenant:           req.Tenant,
		App:              req.App,
		Model:            req.Model,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
		CostUSD:          costUSD,
		Metadata:         req.Metadata,
	}

	saved, err := store.RecordUsage(r.Context(), event)
	if err != nil {
		h.logger.Error("record usage", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Prometheus counters
	tenant := req.Tenant
	if tenant == "" {
		tenant = "_"
	}
	metrics.UsageEventsTotal.WithLabelValues(req.Model, tenant).Inc()
	metrics.TokensTotal.WithLabelValues(req.Model, "prompt").Add(float64(req.PromptTokens))
	metrics.TokensTotal.WithLabelValues(req.Model, "completion").Add(float64(req.CompletionTokens))
	if costUSD > 0 {
		metrics.CostTotalUSD.WithLabelValues(req.Model).Add(costUSD)
	}

	writeJSON(w, http.StatusCreated, saved)
}

func (h *Handler) ListUsage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenant := q.Get("tenant")
	app := q.Get("app")
	mdl := q.Get("model")

	from := parseTime(q.Get("from"), time.Now().AddDate(0, -1, 0))
	to := parseTime(q.Get("to"), time.Now())

	limit := parseIntDefault(q.Get("limit"), 50)
	offset := parseIntDefault(q.Get("offset"), 0)
	if limit > 500 {
		limit = 500
	}

	events, err := store.ListUsageEvents(r.Context(), tenant, app, mdl, from, to, limit, offset)
	if err != nil {
		h.logger.Error("list usage", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if events == nil {
		events = []model.UsageEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// ── Cost Reports ──────────────────────────────────────────────────────────────

func (h *Handler) CostReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	groupBy := model.GroupBy(q.Get("group_by"))
	switch groupBy {
	case model.GroupByTenant, model.GroupByApp, model.GroupByModel:
		// valid
	default:
		groupBy = model.GroupByModel
	}

	from := parseTime(q.Get("from"), time.Now().AddDate(0, -1, 0))
	to := parseTime(q.Get("to"), time.Now())

	report, err := store.BuildCostReport(r.Context(), groupBy, from, to)
	if err != nil {
		h.logger.Error("build cost report", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if report.Groups == nil {
		report.Groups = []model.CostGroup{}
	}
	writeJSON(w, http.StatusOK, report)
}

// ListModels returns all known models with their pricing.
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.pricing.All())
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return fallback
}

func parseIntDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v >= 0 {
		return v
	}
	return def
}
