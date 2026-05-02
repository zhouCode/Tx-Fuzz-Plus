package replay

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/MariusVanDerWijden/tx-fuzz/interestingness"
	"github.com/MariusVanDerWijden/tx-fuzz/runner"
)

type EnvironmentSpec struct {
	RPCURL      string `json:"rpc_url,omitempty"`
	ClientLabel string `json:"client_label"`
	ForkLabel   string `json:"fork_label,omitempty"`
	ChainID     string `json:"chain_id,omitempty"`
}

type Bundle struct {
	BundleVersion    string                          `json:"bundle_version"`
	Case             runner.TestcaseRecord           `json:"case"`
	ExpectedFeedback feedback.Record                 `json:"expected_feedback"`
	Signature        interestingness.SignatureRecord `json:"signature"`
	ReplayCommand    []string                        `json:"replay_command"`
	Environment      EnvironmentSpec                 `json:"environment"`
}

func ExportBundle(root string, retained runner.RetainedCase, env EnvironmentSpec) (string, error) {
	dir := filepath.Join(root, retained.Case.CaseID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	bundle := Bundle{
		BundleVersion:    "v1",
		Case:             retained.Case,
		ExpectedFeedback: retained.Feedback,
		Signature:        retained.Signature,
		ReplayCommand:    []string{"tx-fuzz", "replay", "--bundle", filepath.Join(dir, "bundle.json")},
		Environment:      env,
	}
	bundlePath := filepath.Join(dir, "bundle.json")
	blob, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(bundlePath, blob, 0o644); err != nil {
		return "", err
	}
	if retained.Case.SignedTxHex != "" {
		raw := retained.Case.SignedTxHex
		if len(raw) >= 2 && raw[:2] == "0x" {
			raw = raw[2:]
		}
		bytes, err := hex.DecodeString(raw)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, "tx.rlp"), bytes, 0o644); err != nil {
			return "", err
		}
	} else {
		if err := os.WriteFile(filepath.Join(dir, "tx.rlp"), []byte{}, 0o644); err != nil {
			return "", err
		}
	}
	return bundlePath, nil
}

func LoadBundle(path string) (Bundle, error) {
	var bundle Bundle
	blob, err := os.ReadFile(path)
	if err != nil {
		return bundle, err
	}
	err = json.Unmarshal(blob, &bundle)
	return bundle, err
}
