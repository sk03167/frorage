package billing

import (
	"testing"

	"frorage/apps/api/internal/store"
)

func TestSummarizeAppliesMargin(t *testing.T) {
	summary := Summarize([]store.UsageEvent{
		{Metric: "egress_byte", Quantity: 1024 * 1024 * 1024},
	}, store.ProviderCost{EgressGB: 100000, MarginBps: 500})

	if summary.SubtotalMicros != 100000 {
		t.Fatalf("expected subtotal 100000, got %d", summary.SubtotalMicros)
	}
	if summary.MarginMicros != 5000 {
		t.Fatalf("expected margin 5000, got %d", summary.MarginMicros)
	}
	if summary.TotalMicros != 105000 {
		t.Fatalf("expected total 105000, got %d", summary.TotalMicros)
	}
}
