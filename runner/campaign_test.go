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
