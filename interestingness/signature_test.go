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

func TestNormalizeBlobAndAuthErrorsKeepDistinctFamilies(t *testing.T) {
	rec := feedback.Record{
		CaseID:          "same",
		SendStatus:      "rpc_error",
		RPCErrorClass:   "insufficient_funds",
		RPCErrorMessage: "blob transaction rejected: insufficient funds for 0xabc tx 0xdef",
	}
	blobSig := SignatureForFeedback("blob", "cancun", rec)
	authSig := SignatureForFeedback("pectra", "prague", rec)
	if blobSig.StableKey == authSig.StableKey {
		t.Fatalf("expected family/fork aware signatures to differ")
	}
	if blobSig.SignatureKind != "rpc_error" || authSig.SignatureKind != "rpc_error" {
		t.Fatalf("expected rpc_error signatures, got blob=%s auth=%s", blobSig.SignatureKind, authSig.SignatureKind)
	}
}
