package corpus

import (
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/MariusVanDerWijden/tx-fuzz/interestingness"
	"github.com/MariusVanDerWijden/tx-fuzz/runner"
)

func TestStoreRetainsAndReplacesByScore(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, 1)
	base := runner.RetainedCase{
		Case:       runner.TestcaseRecord{CaseID: "case-a", CampaignID: "campaign-1"},
		Feedback:   feedback.Record{CaseID: "case-a", SendStatus: "rpc_error", RPCErrorClass: "nonce_too_low"},
		Signature:  interestingness.SignatureRecord{StableKey: "sig-1", SignatureKind: "rpc_error"},
		Score:      80,
		Reasons:    []string{"new_rpc_error"},
		RetainedAt: time.Unix(1700000003, 0).UTC(),
	}
	decision := store.Decide(base)
	if !decision.Retain || decision.Duplicate {
		t.Fatalf("unexpected decision for first retain: %#v", decision)
	}
	if err := store.Save(base); err != nil {
		t.Fatalf("save first case: %v", err)
	}

	lower := base
	lower.Case.CaseID = "case-b"
	lower.Score = 70
	decision = store.Decide(lower)
	if decision.Retain || !decision.Duplicate {
		t.Fatalf("expected lower-score duplicate to be skipped: %#v", decision)
	}

	higher := base
	higher.Case.CaseID = "case-c"
	higher.Score = 95
	decision = store.Decide(higher)
	if !decision.Retain || !decision.Duplicate {
		t.Fatalf("expected higher-score duplicate to replace: %#v", decision)
	}
	if err := store.Save(higher); err != nil {
		t.Fatalf("save replacement case: %v", err)
	}

	entries := store.Entries("sig-1")
	if len(entries) != 1 || entries[0].Case.CaseID != "case-c" {
		t.Fatalf("unexpected retained entries: %#v", entries)
	}
}
