package spammer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/corpus"
	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/MariusVanDerWijden/tx-fuzz/interestingness"
	"github.com/MariusVanDerWijden/tx-fuzz/runner"
)

func TestCampaignSinkSkipsCanonicalFeedbackForNonTerminalPendingOutcome(t *testing.T) {
	root := t.TempDir()
	sink := &campaignSink{
		artifactRoot: root,
		replayDir:    filepath.Join(root, "replay"),
		store:        corpus.NewStore(filepath.Join(root, "retain"), 1),
	}
	outcome := runner.Outcome{
		Case: runner.TestcaseRecord{
			CaseID:       "case-1",
			CampaignID:   "campaign-1",
			RunStartedAt: time.Unix(1700000000, 0).UTC(),
			Sequence:     1,
			TxFamily:     "basic",
			ForkLabel:    "cancun",
		},
		Feedback: feedback.Record{
			CaseID:       "case-1",
			SendStatus:   "sent",
			AnomalyFlags: []string{"receipt_timeout"},
		},
		Signature: interestingness.SignatureRecord{StableKey: "sig"},
	}

	if _, err := sink.Accept(context.Background(), outcome); err != nil {
		t.Fatalf("sink accept: %v", err)
	}

	casePath := filepath.Join(root, "cases", "case-1.json")
	if _, err := os.Stat(casePath); err != nil {
		t.Fatalf("expected case artifact: %v", err)
	}
	feedbackPath := filepath.Join(root, "feedback", "case-1.json")
	if _, err := os.Stat(feedbackPath); !os.IsNotExist(err) {
		t.Fatalf("expected nonterminal feedback to be withheld, stat err=%v", err)
	}
}

func TestCampaignSinkWritesCanonicalFeedbackForTerminalReceiptOutcome(t *testing.T) {
	root := t.TempDir()
	sink := &campaignSink{
		artifactRoot: root,
		replayDir:    filepath.Join(root, "replay"),
		store:        corpus.NewStore(filepath.Join(root, "retain"), 1),
	}
	status := uint64(1)
	latency := int64(12)
	outcome := runner.Outcome{
		Case: runner.TestcaseRecord{
			CaseID:       "case-2",
			CampaignID:   "campaign-1",
			RunStartedAt: time.Unix(1700000000, 0).UTC(),
			Sequence:     2,
			TxFamily:     "basic",
			ForkLabel:    "cancun",
		},
		Feedback: feedback.Record{
			CaseID:             "case-2",
			SendStatus:         "sent",
			ReceiptObserved:    true,
			ReceiptStatus:      &status,
			InclusionLatencyMS: &latency,
			ExecutionBucket:    "success",
		},
		Signature: interestingness.SignatureRecord{StableKey: "sig"},
	}

	if _, err := sink.Accept(context.Background(), outcome); err != nil {
		t.Fatalf("sink accept: %v", err)
	}

	feedbackPath := filepath.Join(root, "feedback", "case-2.json")
	if _, err := os.Stat(feedbackPath); err != nil {
		t.Fatalf("expected terminal feedback artifact: %v", err)
	}
}
