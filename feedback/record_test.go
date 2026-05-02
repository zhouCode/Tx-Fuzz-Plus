package feedback

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFeedbackRecordJSONRoundTrip(t *testing.T) {
	t0 := time.Unix(1700000001, 0).UTC()
	status := uint64(1)
	gas := uint64(21000)
	latency := int64(125)
	record := Record{
		CaseID:             "case-001",
		RPCLabel:           "local",
		SubmitStartedAt:    t0,
		SubmitLatencyMS:    5,
		SendStatus:         "sent",
		RPCErrorClass:      "",
		ReceiptObserved:    true,
		ReceiptStatus:      &status,
		ReceiptGasUsed:     &gas,
		InclusionLatencyMS: &latency,
		ExecutionBucket:    "success",
		AnomalyFlags:       []string{"low_latency"},
		ProcessSignals: ProcessSignal{
			ExitObserved: true,
			StderrHints:  []string{"hint"},
		},
	}
	blob, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal feedback: %v", err)
	}
	var got Record
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal feedback: %v", err)
	}
	if got.CaseID != record.CaseID || !got.ReceiptObserved || got.ExecutionBucket != "success" {
		t.Fatalf("unexpected roundtrip result: %#v", got)
	}
}
