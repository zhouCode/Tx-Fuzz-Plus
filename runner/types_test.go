package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
)

func TestTestcaseRecordJSONRoundTrip(t *testing.T) {
	t0 := time.Unix(1700000000, 0).UTC()
	record := TestcaseRecord{
		CaseID:             "case-001",
		CampaignID:         "campaign-001",
		RunStartedAt:       t0,
		Sequence:           1,
		TxFamily:           "basic",
		ForkLabel:          "cancun",
		SourceKind:         "corpus",
		Seed:               42,
		CorpusInputRef:     "input-a",
		Sender:             "0xabc",
		Nonce:              7,
		GasLimit:           21000,
		AccessListEnabled:  true,
		ValueWei:           "1",
		FeeFields:          map[string]string{"gas_price": "100"},
		BlobCount:          2,
		AuthorizationCount: 1,
		LaneID:             "lane-a",
		SenderShard:        "shard-0",
		ConsumedNonceDomains: []string{
			"0xabc",
			"0xdef",
		},
		Mutation: MutationRecord{
			BaseInputHash: "base",
			MutatorNames:  []string{"byte-slice-mutation"},
			MutationCount: 1,
			FieldHints:    []string{"calldata"},
		},
		UnsignedSummary: TxSummary{
			To:               "0xdef",
			ContractCreation: false,
			DataLen:          16,
			AccessListSize:   2,
			TxType:           2,
		},
		SignedTxHex:  "0xdeadbeef",
		SignedTxHash: "0xbead",
	}

	blob, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal testcase: %v", err)
	}

	var got TestcaseRecord
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal testcase: %v", err)
	}

	if got.CaseID != record.CaseID || got.TxFamily != record.TxFamily || got.Seed != record.Seed {
		t.Fatalf("unexpected roundtrip result: %#v", got)
	}
	if got.BlobCount != record.BlobCount || got.AuthorizationCount != record.AuthorizationCount {
		t.Fatalf("family counts not preserved: %#v", got)
	}
	if got.LaneID != record.LaneID || got.SenderShard != record.SenderShard || len(got.ConsumedNonceDomains) != 2 {
		t.Fatalf("v2 metadata not preserved: %#v", got)
	}
	if got.Mutation.BaseInputHash != record.Mutation.BaseInputHash || len(got.Mutation.MutatorNames) != 1 {
		t.Fatalf("mutation not preserved: %#v", got.Mutation)
	}
}

func TestWriteCaseArtifactPreservesFlatCasePathAndAdditiveMetadata(t *testing.T) {
	root := t.TempDir()
	record := TestcaseRecord{
		CaseID:               "case-001",
		CampaignID:           "campaign-001",
		RunStartedAt:         time.Unix(1700000000, 0).UTC(),
		Sequence:             1,
		TxFamily:             "basic",
		ForkLabel:            "cancun",
		SourceKind:           "corpus",
		Seed:                 42,
		Sender:               "0xabc",
		Nonce:                7,
		GasLimit:             21000,
		AccessListEnabled:    true,
		ValueWei:             "1",
		FeeFields:            map[string]string{"gas_price": "100"},
		LaneID:               "lane-a",
		SenderShard:          "shard-0",
		ConsumedNonceDomains: []string{"0xabc", "0xdef"},
		SignedTxHex:          "0xdeadbeef",
		SignedTxHash:         "0xbead",
	}
	fb := feedback.Record{CaseID: record.CaseID, SendStatus: "sent"}

	casePath, feedbackPath, err := WriteCaseArtifact(root, record, fb)
	if err != nil {
		t.Fatalf("write case artifact: %v", err)
	}
	if want := filepath.Join(root, "cases", record.CaseID+".json"); casePath != want {
		t.Fatalf("unexpected case path: got %q want %q", casePath, want)
	}
	if want := filepath.Join(root, "feedback", record.CaseID+".json"); feedbackPath != want {
		t.Fatalf("unexpected feedback path: got %q want %q", feedbackPath, want)
	}

	blob, err := os.ReadFile(casePath)
	if err != nil {
		t.Fatalf("read case artifact: %v", err)
	}
	var got TestcaseRecord
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal case artifact: %v", err)
	}
	if got.CaseID != record.CaseID || got.TxFamily != record.TxFamily || got.SignedTxHash != record.SignedTxHash {
		t.Fatalf("legacy fields not preserved: %#v", got)
	}
	if got.LaneID != record.LaneID || got.SenderShard != record.SenderShard {
		t.Fatalf("additive metadata not preserved: %#v", got)
	}
	if len(got.ConsumedNonceDomains) != len(record.ConsumedNonceDomains) {
		t.Fatalf("nonce domains not preserved: %#v", got.ConsumedNonceDomains)
	}
}

type configDefaultBuilder struct{}

func (configDefaultBuilder) Build(_ context.Context, sequence int) (*Case, error) {
	return &Case{Record: TestcaseRecord{Sequence: sequence}}, nil
}

type configDefaultSubmitter struct{}

func (configDefaultSubmitter) Submit(_ context.Context, _ *Case) (feedback.Record, error) {
	return feedback.Record{SendStatus: "rpc_error", RPCErrorClass: "rpc_error", RPCErrorMessage: "boom"}, nil
}

type capturingSink struct{ outcome Outcome }

func (s *capturingSink) Accept(_ context.Context, outcome Outcome) (SinkResult, error) {
	s.outcome = outcome
	return SinkResult{}, nil
}

func TestRunCampaignDefaultsFamilyMetadataFromConfig(t *testing.T) {
	sink := &capturingSink{}
	stats, err := RunCampaign(context.Background(), Config{
		CampaignID: "campaign-blob",
		Cases:      1,
		TxFamily:   "blob",
		ForkLabel:  "cancun",
	}, configDefaultBuilder{}, configDefaultSubmitter{}, sink)
	if err != nil {
		t.Fatalf("run campaign: %v", err)
	}
	if stats.TotalCases != 1 || stats.TxFamily != "blob" {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if sink.outcome.Case.CampaignID != "campaign-blob" || sink.outcome.Case.TxFamily != "blob" || sink.outcome.Case.ForkLabel != "cancun" {
		t.Fatalf("config defaults not applied: %#v", sink.outcome.Case)
	}
	if sink.outcome.Feedback.CaseID != sink.outcome.Case.CaseID {
		t.Fatalf("feedback case id not backfilled: feedback=%#v case=%#v", sink.outcome.Feedback, sink.outcome.Case)
	}
	if sink.outcome.Case.RunStartedAt.IsZero() {
		t.Fatalf("expected run start to be populated")
	}
}

func TestWriteCaseArtifactPreservesCanonicalFlatPaths(t *testing.T) {
	root := t.TempDir()
	record := TestcaseRecord{
		CaseID:       "case-compat-001",
		CampaignID:   "campaign-001",
		RunStartedAt: time.Unix(1700000000, 0).UTC(),
		Sequence:     1,
		TxFamily:     "basic",
		ForkLabel:    "cancun",
	}
	fb := feedback.Record{
		CaseID:     record.CaseID,
		RPCLabel:   "local",
		SendStatus: "sent",
	}

	casePath, feedbackPath, err := WriteCaseArtifact(root, record, fb)
	if err != nil {
		t.Fatalf("write case artifact: %v", err)
	}

	wantCasePath := filepath.Join(root, "cases", record.CaseID+".json")
	wantFeedbackPath := filepath.Join(root, "feedback", record.CaseID+".json")
	if casePath != wantCasePath {
		t.Fatalf("case path changed: got %q want %q", casePath, wantCasePath)
	}
	if feedbackPath != wantFeedbackPath {
		t.Fatalf("feedback path changed: got %q want %q", feedbackPath, wantFeedbackPath)
	}

	caseBlob, err := os.ReadFile(casePath)
	if err != nil {
		t.Fatalf("read case artifact: %v", err)
	}
	var gotCase TestcaseRecord
	if err := json.Unmarshal(caseBlob, &gotCase); err != nil {
		t.Fatalf("unmarshal case artifact: %v", err)
	}
	if gotCase.CaseID != record.CaseID || gotCase.CampaignID != record.CampaignID {
		t.Fatalf("unexpected case artifact contents: %#v", gotCase)
	}

	feedbackBlob, err := os.ReadFile(feedbackPath)
	if err != nil {
		t.Fatalf("read feedback artifact: %v", err)
	}
	var gotFeedback feedback.Record
	if err := json.Unmarshal(feedbackBlob, &gotFeedback); err != nil {
		t.Fatalf("unmarshal feedback artifact: %v", err)
	}
	if gotFeedback.CaseID != fb.CaseID || gotFeedback.SendStatus != fb.SendStatus {
		t.Fatalf("unexpected feedback artifact contents: %#v", gotFeedback)
	}
}
