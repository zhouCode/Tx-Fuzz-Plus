package spammer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestResolveConfiguredAccountsFromLocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")
	entries := []AccountEntry{
		{
			Address:    "0x8943545177806ED17B9F23F0a21ee5948eCaa776",
			PrivateKey: "bcdf20249abf0ed6d944c0288fad489e33f66b3960d9e6229c1cd214ed3bbe31",
		},
		{
			Address:    "0xE25583099BA105D9ec0A67f5Ae86D90e50036425",
			PrivateKey: "39725efee3fb28614de3bacaffe4cc4bd8c436257e2c8bb887c4b5c4be45e76d",
		},
	}
	blob, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal entries: %v", err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatalf("write entries: %v", err)
	}

	faucet, keys, source, err := resolveConfiguredAccounts(path, 2)
	if err != nil {
		t.Fatalf("resolve configured accounts: %v", err)
	}
	if source != path {
		t.Fatalf("unexpected source path: %s", source)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if faucet == nil {
		t.Fatalf("expected faucet key")
	}
	gotAddr := crypto.PubkeyToAddress(faucet.PublicKey).Hex()
	if gotAddr != entries[0].Address {
		t.Fatalf("expected faucet address %s, got %s", entries[0].Address, gotAddr)
	}
}
