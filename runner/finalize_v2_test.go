package runner

import (
	"context"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/ethereum/go-ethereum/core/types"
)

type stubReceiptAwaiter struct {
	record feedback.Record
}

func (s stubReceiptAwaiter) AwaitReceipt(context.Context, *Case) (feedback.Record, error) {
	return s.record, nil
}

type stubPending struct {
	record feedback.Record
}

func (s stubPending) Await(context.Context) (feedback.Record, error) {
	return s.record, nil
}

type stubSink struct {
	outcomes []Outcome
}

func (s *stubSink) Accept(_ context.Context, outcome Outcome) (SinkResult, error) {
	s.outcomes = append(s.outcomes, outcome)
	return SinkResult{}, nil
}

func TestDrainOneFinalizedCaseSendFailureStaysConsistent(t *testing.T) {
	log := NewEventLog(t.TempDir())
	report := newV2Report(Config{CampaignID: "c1", TxFamily: "basic"}, V2Options{LaneID: "lane-0", RPCLabel: "rpc-a", SendArchitecture: "v2-single-lane"})
	sink := &stubSink{}
	start := time.Unix(1700000000, 0).UTC()
	pending, err := drainOneFinalizedCase(context.Background(), []pendingCase{{
		record: TestcaseRecord{CaseID: "case-1", LaneID: "lane-0", SignedTxHash: "0x1", RunStartedAt: start},
		tx:     &Case{Record: TestcaseRecord{CaseID: "case-1", LaneID: "lane-0"}, Tx: types.NewTx(&types.LegacyTx{})},
		pending: stubPending{record: feedback.Record{
			CaseID:          "case-1",
			SendStatus:      "rpc_error",
			SendState:       "rpc_error",
			RPCErrorClass:   "rpc_error",
			RPCErrorMessage: "intrinsic gas too low",
		}},
	}}, sink, &Stats{}, report, NewInFlightLimiter(1, 1), V2Options{LaneID: "lane-0", EventLog: log}, stubReceiptAwaiter{})
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected pending drained, got %d", len(pending))
	}
	if got := sink.outcomes[0].Feedback; got.SendState != "rpc_error" || got.ConfirmState != "" || got.ReceiptObserved {
		t.Fatalf("unexpected feedback: %#v", got)
	}
	records, err := ReadEventLog(log.Path())
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if len(records) != 1 || records[0].EventType != EventTypeSendRejected {
		t.Fatalf("unexpected event records: %#v", records)
	}
	if report.ConfirmStats.TerminalCases != 0 {
		t.Fatalf("send failure must not count as terminal confirm case: %#v", report.ConfirmStats)
	}
}

func TestDrainOneFinalizedCaseAwaitingReceiptIsNonTerminal(t *testing.T) {
	report := newV2Report(Config{CampaignID: "c1", TxFamily: "basic"}, V2Options{LaneID: "lane-0", RPCLabel: "rpc-a", SendArchitecture: "v2-single-lane"})
	sink := &stubSink{}
	start := time.Now().Add(-2 * time.Second)
	_, err := drainOneFinalizedCase(context.Background(), []pendingCase{{
		record: TestcaseRecord{CaseID: "case-2", LaneID: "lane-0", SignedTxHash: "0x2", RunStartedAt: start},
		tx:     &Case{Record: TestcaseRecord{CaseID: "case-2", LaneID: "lane-0"}, Tx: types.NewTx(&types.LegacyTx{})},
		pending: stubPending{record: feedback.Record{
			CaseID:     "case-2",
			SendStatus: "sent",
			SendState:  "sent",
		}},
	}}, sink, &Stats{}, report, NewInFlightLimiter(1, 1), V2Options{
		LaneID:         "lane-0",
		ConfirmSLA:     10 * time.Second,
		ConfirmDrain:   30 * time.Second,
		ReceiptTimeout: time.Second,
	}, stubReceiptAwaiter{record: feedback.Record{AnomalyFlags: []string{"receipt_timeout"}}})
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	got := sink.outcomes[0].Feedback
	if got.ConfirmState != string(ConfirmStateAwaitingReceipt) {
		t.Fatalf("expected awaiting_receipt, got %#v", got)
	}
	if report.ConfirmStats.TerminalCases != 0 {
		t.Fatalf("awaiting_receipt must not count as terminal: %#v", report.ConfirmStats)
	}
}

func TestDrainOneFinalizedCaseMarksSLABreachAndShutdownByBudgets(t *testing.T) {
	report := newV2Report(Config{CampaignID: "c1", TxFamily: "basic"}, V2Options{LaneID: "lane-0", RPCLabel: "rpc-a", SendArchitecture: "v2-single-lane"})
	sink := &stubSink{}
	start := time.Now().Add(-70 * time.Second)
	_, err := drainOneFinalizedCase(context.Background(), []pendingCase{{
		record: TestcaseRecord{CaseID: "case-3", LaneID: "lane-0", SignedTxHash: "0x3", RunStartedAt: start},
		tx:     &Case{Record: TestcaseRecord{CaseID: "case-3", LaneID: "lane-0"}, Tx: types.NewTx(&types.LegacyTx{})},
		pending: stubPending{record: feedback.Record{
			CaseID:     "case-3",
			SendStatus: "sent",
			SendState:  "sent",
		}},
	}}, sink, &Stats{}, report, NewInFlightLimiter(1, 1), V2Options{
		LaneID:         "lane-0",
		ConfirmSLA:     2 * time.Second,
		ConfirmDrain:   time.Second,
		ReceiptTimeout: time.Second,
	}, stubReceiptAwaiter{record: feedback.Record{AnomalyFlags: []string{"receipt_timeout"}}})
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	got := sink.outcomes[0].Feedback
	if got.ConfirmState != string(ConfirmStateUnresolvedShutdown) {
		t.Fatalf("expected unresolved_shutdown, got %#v", got)
	}
	if !got.SoftDeadlineBreached {
		t.Fatalf("expected soft deadline breach: %#v", got)
	}
}
