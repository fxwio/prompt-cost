// Package model defines domain types for the Prompt & Cost Platform.
package model

import "time"

// ── Prompt Templates ──────────────────────────────────────────────────────────

// Template is a named prompt template with multiple versions.
type Template struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Tags          []string  `json:"tags"`
	ActiveVersion int       `json:"active_version"` // 0 = no active version
	VersionCount  int       `json:"version_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TemplateVersion is one immutable snapshot of a template's content.
// Content uses Go text/template syntax: {{.variable_name}}.
type TemplateVersion struct {
	ID         string    `json:"id"`
	TemplateID string    `json:"template_id"`
	Version    int       `json:"version"`
	Content    string    `json:"content"`
	Variables  []string  `json:"variables"` // names extracted from {{.X}} expressions
	CreatedAt  time.Time `json:"created_at"`
}

// RenderRequest carries variables for a render call.
type RenderRequest struct {
	Variables map[string]string `json:"variables"`
	Version   int               `json:"version,omitempty"` // 0 = use active
}

// RenderResponse is the rendered prompt.
type RenderResponse struct {
	Rendered  string   `json:"rendered"`
	Version   int      `json:"version"`
	Variables []string `json:"variables"`
}

// ── Usage & Cost ──────────────────────────────────────────────────────────────

// UsageEvent records one LLM API call.
type UsageEvent struct {
	ID               string            `json:"id"`
	Tenant           string            `json:"tenant"`
	App              string            `json:"app"`
	Model            string            `json:"model"`
	PromptTokens     int               `json:"prompt_tokens"`
	CompletionTokens int               `json:"completion_tokens"`
	CostUSD          float64           `json:"cost_usd"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

// GroupBy defines aggregation dimension for cost reports.
type GroupBy string

const (
	GroupByTenant GroupBy = "tenant"
	GroupByApp    GroupBy = "app"
	GroupByModel  GroupBy = "model"
)

// CostReport aggregates usage events over a time period.
type CostReport struct {
	From             time.Time   `json:"from"`
	To               time.Time   `json:"to"`
	GroupBy          GroupBy     `json:"group_by"`
	TotalEvents      int         `json:"total_events"`
	TotalPromptTokens     int    `json:"total_prompt_tokens"`
	TotalCompletionTokens int    `json:"total_completion_tokens"`
	TotalCostUSD     float64     `json:"total_cost_usd"`
	Groups           []CostGroup `json:"groups"`
}

// CostGroup is one aggregated row in a cost report.
type CostGroup struct {
	Key                   string  `json:"key"`
	EventCount            int     `json:"event_count"`
	PromptTokens          int     `json:"prompt_tokens"`
	CompletionTokens      int     `json:"completion_tokens"`
	TotalTokens           int     `json:"total_tokens"`
	CostUSD               float64 `json:"cost_usd"`
	AvgCostPerEvent       float64 `json:"avg_cost_per_event"`
}

// ── Pricing ───────────────────────────────────────────────────────────────────

// ModelPricing holds per-1M-token prices for a model.
type ModelPricing struct {
	Model          string  `json:"model"`
	Provider       string  `json:"provider"`
	InputPer1M     float64 `json:"input_per_1m_usd"`  // price per 1M input tokens
	OutputPer1M    float64 `json:"output_per_1m_usd"` // price per 1M output tokens
}
