package runner

import (
	"context"
	"encoding/json"
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
	if got.Mutation.BaseInputHash != record.Mutation.BaseInputHash || len(got.Mutation.MutatorNames) != 1 {
		t.Fatalf("mutation not preserved: %#v", got.Mutation)
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
