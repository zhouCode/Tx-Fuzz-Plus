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
	block := uint64(12345)
	observedAt := time.Unix(1700000010, 0).UTC()
	finalizedAt := time.Unix(1700000020, 0).UTC()
	record := Record{
		CaseID:               "case-001",
		RPCLabel:             "local",
		SubmitStartedAt:      t0,
		SubmitLatencyMS:      5,
		SendStatus:           "sent",
		LaneID:               "lane-a",
		SendState:            "sent",
		ConfirmState:         "included_success",
		RPCErrorClass:        "",
		ReceiptObserved:      true,
		ReceiptStatus:        &status,
		ReceiptGasUsed:       &gas,
		ReceiptBlockNumber:   &block,
		InclusionLatencyMS:   &latency,
		LastObservedAt:       &observedAt,
		ExecutionBucket:      "success",
		AnomalyFlags:         []string{"low_latency"},
		ReconciliationCount:  1,
		SoftDeadlineBreached: true,
		FinalizedAt:          &finalizedAt,
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
	if got.LaneID != record.LaneID || got.SendState != record.SendState || got.ConfirmState != record.ConfirmState {
		t.Fatalf("v2 state metadata missing: %#v", got)
	}
	if got.ReceiptBlockNumber == nil || *got.ReceiptBlockNumber != block {
		t.Fatalf("receipt block not preserved: %#v", got.ReceiptBlockNumber)
	}
	if got.LastObservedAt == nil || !got.LastObservedAt.Equal(observedAt) || got.FinalizedAt == nil || !got.FinalizedAt.Equal(finalizedAt) {
		t.Fatalf("observation/finalization times not preserved: %#v", got)
	}
	if got.ReconciliationCount != 1 || !got.SoftDeadlineBreached {
		t.Fatalf("reconciliation metadata not preserved: %#v", got)
	}
}

func TestFeedbackRecordLegacyPayloadRemainsReadableWithoutPhaseAFields(t *testing.T) {
	payload := []byte(`{
  "case_id": "case-legacy",
  "rpc_label": "local",
  "submit_started_at": "2023-11-14T22:13:21Z",
  "submit_latency_ms": 5,
  "send_status": "sent",
  "receipt_observed": false
}`)

	var got Record
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal legacy payload: %v", err)
	}
	if got.CaseID != "case-legacy" || got.SendStatus != "sent" || got.ReceiptObserved {
		t.Fatalf("unexpected legacy payload result: %#v", got)
	}
	if got.LaneID != "" || got.SendState != "" || got.ConfirmState != "" || got.ReceiptBlockNumber != nil || got.LastObservedAt != nil || got.FinalizedAt != nil {
		t.Fatalf("expected additive fields to remain optional: %#v", got)
	}
}
