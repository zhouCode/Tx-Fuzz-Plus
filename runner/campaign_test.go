package runner

import (
	"context"
	"fmt"
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
	submits  int
	asyncs   int
}

func (s *orderedAsyncSubmitter) Submit(ctx context.Context, c *Case) (feedback.Record, error) {
	s.submits++
	*s.events = append(*s.events, fmt.Sprintf("legacy-submit-%d", c.Record.Sequence))
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

func (s *orderedAsyncSubmitter) SubmitAsync(_ context.Context, c *Case) (PendingSubmission, error) {
	s.asyncs++
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

func TestRunCampaignKeepsLegacySequentialSubmitContractEvenWhenSubmitterSupportsAsync(t *testing.T) {
	var events []string
	release1 := make(chan struct{})
	submitter := &orderedAsyncSubmitter{events: &events, release1: release1}
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
		}, orderedBuilder{events: &events}, submitter, orderedSink{events: &events})
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	if contains(events, "build-2") || contains(events, "submit-2") || contains(events, "legacy-submit-2") {
		t.Fatalf("legacy runner should not advance to second case before first finalize; events=%v", events)
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
	if submitter.submits != 2 || submitter.asyncs != 2 {
		t.Fatalf("legacy runner should route through Submit for each case; submits=%d asyncs=%d events=%v", submitter.submits, submitter.asyncs, events)
	}
	if !contains(events, "finalize-1") || !contains(events, "legacy-submit-2") || !contains(events, "build-2") {
		t.Fatalf("legacy runner did not finish sequential progression as expected: %v", events)
	}
	if indexOf(events, "build-2") < indexOf(events, "finalize-1") {
		t.Fatalf("legacy runner advanced to build-2 before finalize-1; events=%v", events)
	}
}

func contains(events []string, want string) bool {
	return indexOf(events, want) >= 0
}

func indexOf(events []string, want string) int {
	for i, event := range events {
		if event == want {
			return i
		}
	}
	return -1
}
