// Package pricing provides LLM model cost calculation.
// Built-in prices sourced from public pricing pages (2025-Q1).
// All prices are in USD per 1M tokens.
package pricing

import (
	"strings"
	"sync"

	"github.com/fxwio/prompt-cost/internal/model"
)

// builtinPrices is the default pricing table for commonly used models.
// Prices in USD per 1M tokens.
var builtinPrices = []model.ModelPricing{
	// OpenAI
	{Model: "gpt-4o", Provider: "openai", InputPer1M: 2.50, OutputPer1M: 10.00},
	{Model: "gpt-4o-2024-11-20", Provider: "openai", InputPer1M: 2.50, OutputPer1M: 10.00},
	{Model: "gpt-4o-mini", Provider: "openai", InputPer1M: 0.15, OutputPer1M: 0.60},
	{Model: "gpt-4o-mini-2024-07-18", Provider: "openai", InputPer1M: 0.15, OutputPer1M: 0.60},
	{Model: "gpt-4-turbo", Provider: "openai", InputPer1M: 10.00, OutputPer1M: 30.00},
	{Model: "gpt-3.5-turbo", Provider: "openai", InputPer1M: 0.50, OutputPer1M: 1.50},
	{Model: "o1", Provider: "openai", InputPer1M: 15.00, OutputPer1M: 60.00},
	{Model: "o1-mini", Provider: "openai", InputPer1M: 3.00, OutputPer1M: 12.00},
	{Model: "o3-mini", Provider: "openai", InputPer1M: 1.10, OutputPer1M: 4.40},
	// Anthropic Claude
	{Model: "claude-3-5-sonnet-20241022", Provider: "anthropic", InputPer1M: 3.00, OutputPer1M: 15.00},
	{Model: "claude-3-5-sonnet-20240620", Provider: "anthropic", InputPer1M: 3.00, OutputPer1M: 15.00},
	{Model: "claude-3-5-haiku-20241022", Provider: "anthropic", InputPer1M: 0.80, OutputPer1M: 4.00},
	{Model: "claude-3-opus-20240229", Provider: "anthropic", InputPer1M: 15.00, OutputPer1M: 75.00},
	{Model: "claude-3-sonnet-20240229", Provider: "anthropic", InputPer1M: 3.00, OutputPer1M: 15.00},
	{Model: "claude-3-haiku-20240307", Provider: "anthropic", InputPer1M: 0.25, OutputPer1M: 1.25},
	// Google Gemini
	{Model: "gemini-1.5-pro", Provider: "google", InputPer1M: 1.25, OutputPer1M: 5.00},
	{Model: "gemini-1.5-flash", Provider: "google", InputPer1M: 0.075, OutputPer1M: 0.30},
	{Model: "gemini-2.0-flash", Provider: "google", InputPer1M: 0.10, OutputPer1M: 0.40},
	// Embeddings
	{Model: "text-embedding-3-small", Provider: "openai", InputPer1M: 0.02, OutputPer1M: 0},
	{Model: "text-embedding-3-large", Provider: "openai", InputPer1M: 0.13, OutputPer1M: 0},
	{Model: "text-embedding-ada-002", Provider: "openai", InputPer1M: 0.10, OutputPer1M: 0},
}

// Table is a queryable pricing table.
type Table struct {
	mu     sync.RWMutex
	prices map[string]model.ModelPricing // keyed by lowercase model name
}

// Default is the global pricing table initialised with built-in prices.
var Default = newTable(builtinPrices)

func newTable(prices []model.ModelPricing) *Table {
	t := &Table{prices: make(map[string]model.ModelPricing, len(prices))}
	for _, p := range prices {
		t.prices[strings.ToLower(p.Model)] = p
	}
	return t
}

// Override replaces or adds a model's pricing entry. Safe for concurrent use.
func (t *Table) Override(p model.ModelPricing) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prices[strings.ToLower(p.Model)] = p
}

// Get returns the pricing for a model. ok is false if the model is unknown.
func (t *Table) Get(modelName string) (model.ModelPricing, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	p, ok := t.prices[strings.ToLower(modelName)]
	return p, ok
}

// Calculate returns the USD cost for the given token counts.
// Returns 0 and false if the model is not in the table.
func (t *Table) Calculate(modelName string, promptTokens, completionTokens int) (float64, bool) {
	p, ok := t.Get(modelName)
	if !ok {
		return 0, false
	}
	cost := (float64(promptTokens)/1_000_000)*p.InputPer1M +
		(float64(completionTokens)/1_000_000)*p.OutputPer1M
	return cost, true
}

// All returns a copy of all pricing entries, sorted by provider then model name.
func (t *Table) All() []model.ModelPricing {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]model.ModelPricing, 0, len(t.prices))
	for _, p := range t.prices {
		out = append(out, p)
	}
	return out
}
