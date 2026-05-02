package spammer

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"os"
	"strings"

	txfuzz "github.com/MariusVanDerWijden/tx-fuzz"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const defaultAccountsPath = ".local/devnet-accounts.json"

type AccountEntry struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
}

func loadAccountEntries(path string) ([]AccountEntry, string, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return nil, "", err
	}
	expanded = strings.TrimSpace(expanded)
	if expanded == "" {
		return nil, "", nil
	}
	blob, err := os.ReadFile(expanded)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, expanded, nil
		}
		return nil, expanded, err
	}
	var entries []AccountEntry
	if err := json.Unmarshal(blob, &entries); err != nil {
		return nil, expanded, err
	}
	return entries, expanded, nil
}

func resolveConfiguredAccounts(rawPath string, requestedCount int) (*ecdsa.PrivateKey, []*ecdsa.PrivateKey, string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		path = defaultAccountsPath
	}
	entries, source, err := loadAccountEntries(path)
	if err != nil {
		return nil, nil, source, err
	}
	if len(entries) == 0 {
		return defaultStaticAccounts(requestedCount), defaultKeyList(requestedCount), source, nil
	}

	if requestedCount <= 0 || requestedCount > len(entries) {
		requestedCount = len(entries)
	}

	keys := make([]*ecdsa.PrivateKey, 0, requestedCount)
	for i := 0; i < requestedCount; i++ {
		entry := entries[i]
		key, err := crypto.ToECDSA(common.FromHex(entry.PrivateKey))
		if err != nil {
			return nil, nil, source, err
		}
		keys = append(keys, key)
	}
	faucet, err := crypto.ToECDSA(common.FromHex(entries[0].PrivateKey))
	if err != nil {
		return nil, nil, source, err
	}
	return faucet, keys, source, nil
}

func defaultStaticAccounts(requestedCount int) *ecdsa.PrivateKey {
	faucet := crypto.ToECDSAUnsafe(common.FromHex(txfuzz.SK))
	return faucet
}

func defaultKeyList(requestedCount int) []*ecdsa.PrivateKey {
	if requestedCount <= 0 || requestedCount > len(staticKeys) {
		requestedCount = len(staticKeys)
	}
	keys := make([]*ecdsa.PrivateKey, 0, requestedCount)
	for i := 0; i < requestedCount; i++ {
		keys = append(keys, crypto.ToECDSAUnsafe(common.FromHex(staticKeys[i])))
	}
	return keys
}
