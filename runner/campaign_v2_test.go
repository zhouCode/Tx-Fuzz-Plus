package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/ethereum/go-ethereum/common"
)

type v2OrderedBuilder struct {
	events *[]string
}

func (b v2OrderedBuilder) Build(_ context.Context, sequence int) (*Case, error) {
	*b.events = append(*b.events, fmt.Sprintf("build-%d", sequence))
	return &Case{Record: TestcaseRecord{
		CaseID:       fmt.Sprintf("case-%d", sequence),
		CampaignID:   "campaign-v2",
		Sequence:     sequence,
		TxFamily:     "basic",
		ForkLabel:    "cancun",
		RunStartedAt: time.Unix(1700000005, 0).UTC(),
		SignedTxHash: common.BigToHash(common.Big1).Hex(),
	}}, nil
}

type v2ControlledSubmitter struct {
	events      *[]string
	finalizeOne chan struct{}
}

func (s v2ControlledSubmitter) Submit(context.Context, *Case) (feedback.Record, error) {
	panic("legacy submit must not be used")
}

func (s v2ControlledSubmitter) SubmitAsync(_ context.Context, c *Case) (PendingSubmission, error) {
	*s.events = append(*s.events, fmt.Sprintf("submit-%d", c.Record.Sequence))
	recordCh := make(chan feedback.Record, 1)
	recordCh <- feedback.Record{CaseID: c.Record.CaseID, SendStatus: "sent", SendState: "sent"}
	return orderedPending{recordCh: recordCh}, nil
}

func (s v2ControlledSubmitter) AwaitReceipt(_ context.Context, c *Case) (feedback.Record, error) {
	if c.Record.Sequence == 1 {
		<-s.finalizeOne
	}
	*s.events = append(*s.events, fmt.Sprintf("finalize-%d", c.Record.Sequence))
	status := uint64(1)
	return feedback.Record{
		CaseID:             c.Record.CaseID,
		SendStatus:         "sent",
		SendState:          "sent",
		ConfirmState:       string(ConfirmStateIncludedSuccess),
		ReceiptObserved:    true,
		ReceiptStatus:      &status,
		ExecutionBucket:    "success",
		SubmitLatencyMS:    1,
		InclusionLatencyMS: ptrInt64(2),
	}, nil
}

func ptrInt64(v int64) *int64 { return &v }

type v2OrderedSink struct {
	events *[]string
}

func (s v2OrderedSink) Accept(_ context.Context, outcome Outcome) (SinkResult, error) {
	*s.events = append(*s.events, fmt.Sprintf("sink-%d-%s", outcome.Case.Sequence, outcome.Feedback.ConfirmState))
	return SinkResult{}, nil
}

func TestRunCampaignV2SubmitsNextCaseBeforeFirstFinalize(t *testing.T) {
	var events []string
	finalizeOne := make(chan struct{})
	done := make(chan struct{})
	var (
		stats Stats
		err   error
	)

	go func() {
		stats, err = RunCampaignV2(context.Background(), Config{
			CampaignID: "campaign-v2",
			Cases:      2,
			TxFamily:   "basic",
			ForkLabel:  "cancun",
		}, v2OrderedBuilder{events: &events}, v2ControlledSubmitter{events: &events, finalizeOne: finalizeOne}, v2OrderedSink{events: &events}, V2Options{})
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		if slices.Contains(events, "submit-2") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected submit-2 before finalize-1; events=%v", events)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if slices.Contains(events, "finalize-1") {
		t.Fatalf("first finalize should still be pending when submit-2 appears; events=%v", events)
	}

	close(finalizeOne)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("RunCampaignV2 did not finish; events=%v", events)
	}
	if err != nil {
		t.Fatalf("RunCampaignV2: %v", err)
	}
	if stats.TotalCases != 2 || stats.SentCases != 2 {
		t.Fatalf("unexpected stats: %#v events=%v", stats, events)
	}
}

func TestRunCampaignV2WritesCanonicalEventLogAndReportSummary(t *testing.T) {
	root := t.TempDir()
	reportPath := filepath.Join(root, "report.json")
	eventLog := NewEventLog(root)
	report := ReportV2{
		CampaignID:       "campaign-v2",
		TxFamily:         "basic",
		TotalCases:       2,
		SentCases:        2,
		RetainedCases:    0,
		DuplicateCases:   0,
		SendArchitecture: "v2-single-lane",
		ThroughputSummary: &ThroughputSummary{
			SubmitDurationMS: 10,
			ConfirmDrainMS:   20,
			TotalDurationMS:  30,
			CasesPerSecond:   200,
		},
	}

	if got := eventLog.Path(); got != filepath.Join(root, "events", "events.jsonl") {
		t.Fatalf("unexpected canonical event log path: %s", got)
	}
	if err := eventLog.Append(EventRecord{At: time.Now().UTC(), CaseID: "case-1", EventType: EventTypeSent, SendState: "sent"}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if err := WriteReportV2(reportPath, report); err != nil {
		t.Fatalf("write report v2: %v", err)
	}
}
