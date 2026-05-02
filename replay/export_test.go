package replay

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/MariusVanDerWijden/tx-fuzz/interestingness"
	"github.com/MariusVanDerWijden/tx-fuzz/runner"
)

func TestExportBundleWritesBundleAndRawTx(t *testing.T) {
	dir := t.TempDir()
	retained := runner.RetainedCase{
		Case: runner.TestcaseRecord{
			CaseID:       "case-1",
			CampaignID:   "campaign-1",
			SignedTxHex:  "0xdeadbeef",
			SignedTxHash: "0xbeef",
		},
		Feedback:   feedback.Record{CaseID: "case-1", SendStatus: "rpc_error", RPCErrorClass: "nonce_too_low"},
		Signature:  interestingness.SignatureRecord{StableKey: "sig-1", SignatureKind: "rpc_error"},
		Score:      80,
		Reasons:    []string{"new_rpc_error"},
		RetainedAt: time.Unix(1700000004, 0).UTC(),
	}
	bundlePath, err := ExportBundle(dir, retained, EnvironmentSpec{ClientLabel: "local", ForkLabel: "cancun"})
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("expected bundle file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(bundlePath), "tx.rlp")); err != nil {
		t.Fatalf("expected tx.rlp file: %v", err)
	}
	bundle, err := LoadBundle(bundlePath)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if len(bundle.ReplayCommand) == 0 || bundle.Case.CaseID != "case-1" {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}
}

func TestExportBundlePreservesFamilyMetadataAndEnvironment(t *testing.T) {
	dir := t.TempDir()
	retained := runner.RetainedCase{
		Case: runner.TestcaseRecord{
			CaseID:             "case-blob-1",
			CampaignID:         "campaign-blob",
			TxFamily:           "blob",
			ForkLabel:          "cancun",
			BlobCount:          2,
			AuthorizationCount: 1,
			SignedTxHex:        "0x01",
		},
		Feedback:   feedback.Record{CaseID: "case-blob-1", SendStatus: "sent"},
		Signature:  interestingness.SignatureRecord{StableKey: "sig-blob", SignatureKind: "receipt"},
		RetainedAt: time.Unix(1700000004, 0).UTC(),
	}
	bundlePath, err := ExportBundle(dir, retained, EnvironmentSpec{
		RPCURL:      "http://127.0.0.1:8545",
		ClientLabel: "reth",
		ForkLabel:   "cancun",
		ChainID:     "1",
	})
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	bundle, err := LoadBundle(bundlePath)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if bundle.Case.TxFamily != "blob" || bundle.Case.BlobCount != 2 || bundle.Case.AuthorizationCount != 1 {
		t.Fatalf("family metadata not preserved: %#v", bundle.Case)
	}
	if bundle.Environment.RPCURL != "http://127.0.0.1:8545" || bundle.Environment.ClientLabel != "reth" || bundle.Environment.ChainID != "1" {
		t.Fatalf("environment not preserved: %#v", bundle.Environment)
	}
}
