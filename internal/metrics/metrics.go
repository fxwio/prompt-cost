// Package metrics registers all Prometheus metrics for prompt-cost.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// UsageEventsTotal counts recorded usage events.
	UsageEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "prompt_cost",
		Name:      "usage_events_total",
		Help:      "Total usage events recorded.",
	}, []string{"model", "tenant"})

	// TokensTotal counts input and output tokens.
	TokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "prompt_cost",
		Name:      "tokens_total",
		Help:      "Total tokens processed.",
	}, []string{"model", "type"}) // type: "prompt" | "completion"

	// CostTotalUSD tracks the running sum of LLM spend.
	CostTotalUSD = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "prompt_cost",
		Name:      "cost_total_usd",
		Help:      "Total USD cost of LLM usage recorded.",
	}, []string{"model"})

	// TemplatesTotal tracks number of templates created.
	TemplatesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "prompt_cost",
		Name:      "templates_total",
		Help:      "Total prompt templates created.",
	})

	// TemplateVersionsTotal tracks template versions created.
	TemplateVersionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "prompt_cost",
		Name:      "template_versions_total",
		Help:      "Total prompt template versions created.",
	})

	// RenderRequestsTotal counts template render calls.
	RenderRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "prompt_cost",
		Name:      "render_requests_total",
		Help:      "Total template render requests.",
	}, []string{"status"}) // "ok" | "error"
)
