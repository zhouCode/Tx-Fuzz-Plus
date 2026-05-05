package runner

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
)

type fakeBuilder struct{}

func (fakeBuilder) Build(_ context.Context, sequence int) (*Case, error) {
	return &Case{Record: TestcaseRecord{CaseID: fmt.Sprintf("case-%d", sequence), CampaignID: "campaign-1", Sequence: sequence, TxFamily: "basic", ForkLabel: "cancun", RunStartedAt: time.Unix(1700000005, 0).UTC()}}, nil
}

type fakeSubmitter struct{}

func (fakeSubmitter) Submit(_ context.Context, c *Case) (feedback.Record, error) {
	if c.Record.Sequence == 1 {
		return feedback.Record{CaseID: c.Record.CaseID, SendStatus: "rpc_error", RPCErrorClass: "nonce_too_low", RPCErrorMessage: "nonce too low 0xabc 1"}, nil
	}
	status := uint64(1)
	return feedback.Record{CaseID: c.Record.CaseID, SendStatus: "sent", ReceiptObserved: true, ReceiptStatus: &status, ExecutionBucket: "success"}, nil
}

type fakeSink struct{ accepted int }

func (s *fakeSink) Accept(_ context.Context, outcome Outcome) (SinkResult, error) {
	s.accepted++
	return SinkResult{Retained: outcome.Score >= 70, Duplicate: outcome.Signature.SignatureKind == "receipt"}, nil
}

func TestRunCampaignCollectsStats(t *testing.T) {
	sink := &fakeSink{}
	stats, err := RunCampaign(context.Background(), Config{CampaignID: "campaign-1", Cases: 3, TxFamily: "basic", ForkLabel: "cancun"}, fakeBuilder{}, fakeSubmitter{}, sink)
	if err != nil {
		t.Fatalf("run campaign: %v", err)
	}
	if sink.accepted != 3 {
		t.Fatalf("expected 3 outcomes, got %d", sink.accepted)
	}
	if stats.TotalCases != 3 || stats.SentCases != 2 || stats.RetainedCases == 0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

type orderedBuilder struct {
	events *[]string
}

func (b orderedBuilder) Build(_ context.Context, sequence int) (*Case, error) {
	*b.events = append(*b.events, fmt.Sprintf("build-%d", sequence))
	return &Case{Record: TestcaseRecord{
		CaseID:       fmt.Sprintf("case-%d", sequence),
		CampaignID:   "campaign-ordered",
		Sequence:     sequence,
		TxFamily:     "basic",
		ForkLabel:    "cancun",
		RunStartedAt: time.Unix(1700000005, 0).UTC(),
	}}, nil
}

type orderedAsyncSubmitter struct {
	events   *[]string
	release1 chan struct{}
}

func (s orderedAsyncSubmitter) Submit(ctx context.Context, c *Case) (feedback.Record, error) {
	pending, err := s.SubmitAsync(ctx, c)
	if err != nil {
		return feedback.Record{}, err
	}
	return pending.Await(ctx)
}

type orderedPending struct {
	recordCh <-chan feedback.Record
}

func (p orderedPending) Await(context.Context) (feedback.Record, error) {
	return <-p.recordCh, nil
}

func (s orderedAsyncSubmitter) SubmitAsync(_ context.Context, c *Case) (PendingSubmission, error) {
	*s.events = append(*s.events, fmt.Sprintf("submit-%d", c.Record.Sequence))
	recordCh := make(chan feedback.Record, 1)
	go func() {
		if c.Record.Sequence == 1 {
			<-s.release1
		}
		*s.events = append(*s.events, fmt.Sprintf("finalize-%d", c.Record.Sequence))
		status := uint64(1)
		recordCh <- feedback.Record{
			CaseID:          c.Record.CaseID,
			SendStatus:      "sent",
			ReceiptObserved: true,
			ReceiptStatus:   &status,
			ExecutionBucket: "success",
		}
	}()
	return orderedPending{recordCh: recordCh}, nil
}

type orderedSink struct {
	events *[]string
}

func (s orderedSink) Accept(_ context.Context, outcome Outcome) (SinkResult, error) {
	*s.events = append(*s.events, fmt.Sprintf("sink-%d-%s", outcome.Case.Sequence, outcome.Feedback.SendStatus))
	return SinkResult{}, nil
}

func TestRunCampaignPrebuildsNextCaseBeforeFirstFinalizeButDelaysSinkUntilFinalized(t *testing.T) {
	var events []string
	release1 := make(chan struct{})
	done := make(chan struct{})
	var (
		stats Stats
		err   error
	)
	go func() {
		stats, err = RunCampaign(context.Background(), Config{
			CampaignID: "campaign-ordered",
			Cases:      2,
			TxFamily:   "basic",
			ForkLabel:  "cancun",
		}, orderedBuilder{events: &events}, orderedAsyncSubmitter{events: &events, release1: release1}, orderedSink{events: &events})
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		if slices.Contains(events, "build-2") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("runner did not prebuild second case before finalize; events=%v", events)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if slices.Contains(events, "sink-1-sent") {
		t.Fatalf("sink should not run before first case finalizes; events=%v", events)
	}

	close(release1)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("run campaign did not complete; events=%v", events)
	}
	if err != nil {
		t.Fatalf("run campaign: %v", err)
	}
	if stats.TotalCases != 2 || stats.SentCases != 2 {
		t.Fatalf("unexpected stats: %#v events=%v", stats, events)
	}
	if !slices.Contains(events, "finalize-1") || !slices.Contains(events, "sink-1-sent") {
		t.Fatalf("missing finalize/sink events: %v", events)
	}
	if slices.Index(events, "sink-1-sent") < slices.Index(events, "finalize-1") {
		t.Fatalf("sink observed case 1 before finalize: %v", events)
	}
}
