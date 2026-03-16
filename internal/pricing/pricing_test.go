package pricing

import (
	"math"
	"testing"

	"github.com/fxwio/prompt-cost/internal/model"
)

func TestCalculate_KnownModel(t *testing.T) {
	// gpt-4o: $2.50/1M input, $10.00/1M output
	cost, ok := Default.Calculate("gpt-4o", 1_000_000, 500_000)
	if !ok {
		t.Fatal("expected ok=true for gpt-4o")
	}
	// 1M input × $2.50 + 0.5M output × $10.00 = $2.50 + $5.00 = $7.50
	want := 7.50
	if math.Abs(cost-want) > 0.0001 {
		t.Errorf("cost = %f, want %f", cost, want)
	}
}

func TestCalculate_UnknownModel(t *testing.T) {
	_, ok := Default.Calculate("not-a-real-model", 100, 100)
	if ok {
		t.Error("expected ok=false for unknown model")
	}
}

func TestCalculate_CaseInsensitive(t *testing.T) {
	cost1, ok1 := Default.Calculate("GPT-4O", 1000, 500)
	cost2, ok2 := Default.Calculate("gpt-4o", 1000, 500)
	if !ok1 || !ok2 {
		t.Fatal("expected ok=true")
	}
	if cost1 != cost2 {
		t.Errorf("case-insensitive mismatch: %f != %f", cost1, cost2)
	}
}

func TestCalculate_EmbeddingModel(t *testing.T) {
	// text-embedding-3-small: $0.02/1M input, $0 output
	cost, ok := Default.Calculate("text-embedding-3-small", 1_000_000, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := 0.02
	if math.Abs(cost-want) > 0.0001 {
		t.Errorf("cost = %f, want %f", cost, want)
	}
}

func TestOverride(t *testing.T) {
	table := newTable(nil)
	table.Override(model.ModelPricing{
		Model:       "custom-model",
		Provider:    "custom",
		InputPer1M:  5.0,
		OutputPer1M: 20.0,
	})
	cost, ok := table.Calculate("custom-model", 100_000, 50_000)
	if !ok {
		t.Fatal("expected ok=true after override")
	}
	// 0.1M × $5 + 0.05M × $20 = $0.50 + $1.00 = $1.50
	want := 1.50
	if math.Abs(cost-want) > 0.0001 {
		t.Errorf("cost = %f, want %f", cost, want)
	}
}

func TestCalculate_SmallTokenCount(t *testing.T) {
	// 1000 prompt tokens + 500 completion tokens with gpt-4o-mini
	// $0.15/1M + $0.60/1M
	cost, ok := Default.Calculate("gpt-4o-mini", 1000, 500)
	if !ok {
		t.Fatal("expected ok=true for gpt-4o-mini")
	}
	// 0.001M × $0.15 + 0.0005M × $0.60 = $0.00015 + $0.00030 = $0.00045
	want := 0.00045
	if math.Abs(cost-want) > 0.000001 {
		t.Errorf("cost = %f, want %f", cost, want)
	}
}

func TestAll_ReturnsEntries(t *testing.T) {
	all := Default.All()
	if len(all) == 0 {
		t.Error("expected non-empty pricing table")
	}
	// Check gpt-4o is in there
	found := false
	for _, p := range all {
		if p.Model == "gpt-4o" {
			found = true
			break
		}
	}
	if !found {
		t.Error("gpt-4o not found in All()")
	}
}
