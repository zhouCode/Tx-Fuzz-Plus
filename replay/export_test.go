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
