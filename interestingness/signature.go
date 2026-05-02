package interestingness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
)

type SignatureRecord struct {
	SignatureVersion string `json:"signature_version"`
	SignatureKind    string `json:"signature_kind"`
	StableKey        string `json:"stable_key"`
	NormalizedText   string `json:"normalized_text"`
}

var (
	hexPattern   = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	noncePattern = regexp.MustCompile(`\b\d+\b`)
)

func SignatureForFeedback(txFamily, forkLabel string, rec feedback.Record) SignatureRecord {
	kind := "anomaly"
	normalized := rec.ExecutionBucket
	switch {
	case rec.SendStatus == "rpc_error":
		kind = "rpc_error"
		normalized = normalizeText(rec.RPCErrorClass + ":" + rec.RPCErrorMessage)
	case rec.ReceiptObserved:
		kind = "receipt"
		status := "none"
		if rec.ReceiptStatus != nil {
			status = fmt.Sprintf("status:%d", *rec.ReceiptStatus)
		}
		normalized = strings.TrimSpace(strings.Join([]string{status, rec.ExecutionBucket, strings.Join(rec.AnomalyFlags, ",")}, "|"))
	default:
		normalized = strings.TrimSpace(strings.Join(rec.AnomalyFlags, ","))
	}
	base := fmt.Sprintf("v1|%s|%s|%s|%s", txFamily, forkLabel, kind, normalized)
	sum := sha256.Sum256([]byte(base))
	return SignatureRecord{
		SignatureVersion: "v1",
		SignatureKind:    kind,
		StableKey:        hex.EncodeToString(sum[:]),
		NormalizedText:   normalized,
	}
}

func ScoreFeedback(sig SignatureRecord, rec feedback.Record) (int, []string) {
	score := 20
	reasons := []string{"baseline"}
	switch sig.SignatureKind {
	case "rpc_error":
		score = 80
		reasons = []string{"new_rpc_error"}
	case "receipt":
		score = 70
		reasons = []string{"receipt_bucket"}
	}
	if rec.ProcessSignals.ExitObserved {
		score = 100
		reasons = []string{"process_signal"}
	}
	return score, reasons
}

func normalizeText(text string) string {
	text = strings.ToLower(text)
	text = hexPattern.ReplaceAllString(text, "<hex>")
	text = noncePattern.ReplaceAllString(text, "<num>")
	text = strings.Join(strings.Fields(text), " ")
	return text
}
