package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/fxwio/prompt-cost/internal/model"
	"github.com/fxwio/prompt-cost/internal/pricing"
)

func newTestHandler() *Handler {
	logger, _ := zap.NewDevelopment()
	return New(pricing.Default, logger)
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestHealth(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d want %d", w.Code, http.StatusOK)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status field: got %v want ok", resp["status"])
	}
}

func TestCreateTemplate_MissingName(t *testing.T) {
	router := NewRouter(newTestHandler(), zap.NewNop())
	w := doRequest(t, router, http.MethodPost, "/templates", map[string]any{
		"description": "no name",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateTemplate_InvalidJSON(t *testing.T) {
	router := NewRouter(newTestHandler(), zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/templates",
		bytes.NewBufferString("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateVersion_NoDB_NotFound(t *testing.T) {
	// Without a database, GetTemplate will panic (pool not init).
	// Test that missing content returns 400 before hitting the DB.
	router := NewRouter(newTestHandler(), zap.NewNop())
	w := doRequest(t, router, http.MethodPost, "/templates/some-id/versions", map[string]any{
		"content": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRenderTemplate_InvalidJSON(t *testing.T) {
	router := NewRouter(newTestHandler(), zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/templates/abc/render",
		bytes.NewBufferString("bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRecordUsage_MissingModel(t *testing.T) {
	router := NewRouter(newTestHandler(), zap.NewNop())
	w := doRequest(t, router, http.MethodPost, "/usage", map[string]any{
		"tenant":            "acme",
		"prompt_tokens":     100,
		"completion_tokens": 50,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRecordUsage_NegativeTokens(t *testing.T) {
	router := NewRouter(newTestHandler(), zap.NewNop())
	w := doRequest(t, router, http.MethodPost, "/usage", map[string]any{
		"model":         "gpt-4o",
		"prompt_tokens": -1,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListModels(t *testing.T) {
	router := NewRouter(newTestHandler(), zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/cost/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d want %d", w.Code, http.StatusOK)
	}
	var models []model.ModelPricing
	if err := json.NewDecoder(w.Body).Decode(&models); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(models) == 0 {
		t.Error("expected at least one model in pricing table")
	}
}

func TestListUsage_DefaultsOK(t *testing.T) {
	// Without DB, this panics. We only test input validation here.
	router := NewRouter(newTestHandler(), zap.NewNop())

	// Test that invalid limit is clamped, not rejected
	req := httptest.NewRequest(http.MethodGet, "/usage?limit=abc", nil)
	w := httptest.NewRecorder()
	// Will panic due to no DB — catch via Recoverer middleware returning 500
	router.ServeHTTP(w, req)
	// We just verify it doesn't return 400 (bad request)
	if w.Code == http.StatusBadRequest {
		t.Errorf("limit=abc should use default, not return 400")
	}
}

func TestActivateVersion_InvalidVersion(t *testing.T) {
	router := NewRouter(newTestHandler(), zap.NewNop())
	req := httptest.NewRequest(http.MethodPut, "/templates/abc/versions/notanumber/activate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}
}
