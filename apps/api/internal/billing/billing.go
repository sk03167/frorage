package billing

import "private-cloud-storage/apps/api/internal/store"

const microsPerDollar = int64(1_000_000)

type UsageSummary struct {
	StorageBytesHours int64 `json:"storageBytesHours"`
	EgressBytes       int64 `json:"egressBytes"`
	Operations        int64 `json:"operations"`
	SubtotalMicros    int64 `json:"subtotalMicros"`
	MarginMicros      int64 `json:"marginMicros"`
	TotalMicros       int64 `json:"totalMicros"`
}

func Summarize(events []store.UsageEvent, cost store.ProviderCost) UsageSummary {
	var summary UsageSummary
	for _, event := range events {
		switch event.Metric {
		case "storage_byte_hour":
			summary.StorageBytesHours += event.Quantity
		case "egress_byte":
			summary.EgressBytes += event.Quantity
		case "object_operation":
			summary.Operations += event.Quantity
		}
	}
	storageMicros := divCeil(summary.StorageBytesHours*cost.StorageGBMonth, bytesInGB*hoursInBillingMonth)
	egressMicros := divCeil(summary.EgressBytes*cost.EgressGB, bytesInGB)
	opsMicros := divCeil(summary.Operations*cost.Operation10K, 10_000)
	summary.SubtotalMicros = storageMicros + egressMicros + opsMicros
	summary.MarginMicros = divCeil(summary.SubtotalMicros*cost.MarginBps, 10_000)
	summary.TotalMicros = summary.SubtotalMicros + summary.MarginMicros
	return summary
}

func divCeil(n, d int64) int64 {
	if n == 0 {
		return 0
	}
	return (n + d - 1) / d
}

const (
	bytesInGB           = int64(1024 * 1024 * 1024)
	hoursInBillingMonth = int64(730)
	_                   = microsPerDollar
)
