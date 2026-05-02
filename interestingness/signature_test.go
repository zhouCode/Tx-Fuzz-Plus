package interestingness

import (
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
)

func TestNormalizeRPCErrorProducesStableSignature(t *testing.T) {
	t0 := time.Unix(1700000002, 0).UTC()
	recA := feedback.Record{
		CaseID:          "a",
		RPCLabel:        "local",
		SubmitStartedAt: t0,
		SendStatus:      "rpc_error",
		RPCErrorClass:   "nonce_too_low",
		RPCErrorMessage: "nonce too low: address 0xabc123 nonce 7 tx 0xdeadbeef",
	}
	recB := feedback.Record{
		CaseID:          "b",
		RPCLabel:        "local",
		SubmitStartedAt: t0,
		SendStatus:      "rpc_error",
		RPCErrorClass:   "nonce_too_low",
		RPCErrorMessage: "nonce too low: address 0xffee22 nonce 999 tx 0xcafebabe",
	}

	sigA := SignatureForFeedback("basic", "cancun", recA)
	sigB := SignatureForFeedback("basic", "cancun", recB)
	if sigA.StableKey != sigB.StableKey {
		t.Fatalf("expected stable keys to match: %q vs %q", sigA.StableKey, sigB.StableKey)
	}
	if sigA.NormalizedText == recA.RPCErrorMessage {
		t.Fatalf("expected normalized text to differ from raw message")
	}
}

func TestReceiptAndErrorSignaturesDoNotCollide(t *testing.T) {
	status := uint64(1)
	receiptSig := SignatureForFeedback("basic", "cancun", feedback.Record{SendStatus: "sent", ReceiptObserved: true, ReceiptStatus: &status, ExecutionBucket: "success"})
	errSig := SignatureForFeedback("basic", "cancun", feedback.Record{SendStatus: "rpc_error", RPCErrorClass: "replacement_underpriced", RPCErrorMessage: "replacement transaction underpriced"})
	if receiptSig.StableKey == errSig.StableKey {
		t.Fatalf("expected distinct signature keys")
	}
}

func TestSignatureIncludesTxFamilyAndForkLabel(t *testing.T) {
	rec := feedback.Record{SendStatus: "rpc_error", RPCErrorClass: "nonce_too_low", RPCErrorMessage: "nonce too low 0xabc 7"}
	basic := SignatureForFeedback("basic", "cancun", rec)
	blob := SignatureForFeedback("blob", "cancun", rec)
	prague := SignatureForFeedback("blob", "prague", rec)
	if basic.StableKey == blob.StableKey {
		t.Fatalf("expected tx family to affect stable key")
	}
	if blob.StableKey == prague.StableKey {
		t.Fatalf("expected fork label to affect stable key")
	}
}
