package runner

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTestcaseRecordJSONRoundTrip(t *testing.T) {
	t0 := time.Unix(1700000000, 0).UTC()
	record := TestcaseRecord{
		CaseID:            "case-001",
		CampaignID:        "campaign-001",
		RunStartedAt:      t0,
		Sequence:          1,
		TxFamily:          "basic",
		ForkLabel:         "cancun",
		SourceKind:        "corpus",
		Seed:              42,
		CorpusInputRef:    "input-a",
		Sender:            "0xabc",
		Nonce:             7,
		GasLimit:          21000,
		AccessListEnabled: true,
		ValueWei:          "1",
		FeeFields:         map[string]string{"gas_price": "100"},
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
	if got.Mutation.BaseInputHash != record.Mutation.BaseInputHash || len(got.Mutation.MutatorNames) != 1 {
		t.Fatalf("mutation not preserved: %#v", got.Mutation)
	}
}

func TestTestcaseRecordJSONRoundTripPreservesFamilyFields(t *testing.T) {
	record := TestcaseRecord{
		CaseID:             "case-family",
		TxFamily:           "blob",
		ForkLabel:          "cancun",
		BlobCount:          2,
		AuthorizationCount: 1,
		FeeFields:          map[string]string{"blob_fee_cap": "123", "gas_fee_cap": "456"},
	}
	blob, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal testcase: %v", err)
	}
	var got TestcaseRecord
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal testcase: %v", err)
	}
	if got.BlobCount != 2 || got.AuthorizationCount != 1 {
		t.Fatalf("family fields not preserved: %#v", got)
	}
	if got.FeeFields["blob_fee_cap"] != "123" {
		t.Fatalf("expected blob fee field, got %#v", got.FeeFields)
	}
}
