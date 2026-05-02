package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/runner"
)

type Decision struct {
	Retain    bool
	Duplicate bool
}

type Store struct {
	root  string
	cap   int
	bySig map[string][]runner.RetainedCase
}

func NewStore(root string, capPerSignature int) *Store {
	return &Store{root: root, cap: capPerSignature, bySig: make(map[string][]runner.RetainedCase)}
}

func (s *Store) Decide(candidate runner.RetainedCase) Decision {
	entries := s.bySig[candidate.Signature.StableKey]
	if len(entries) == 0 {
		return Decision{Retain: true}
	}
	best := entries[0]
	if candidate.Score > best.Score {
		return Decision{Retain: true, Duplicate: true}
	}
	if len(entries) < s.cap {
		return Decision{Retain: true, Duplicate: true}
	}
	return Decision{Retain: false, Duplicate: true}
}

func (s *Store) Save(candidate runner.RetainedCase) error {
	candidate.RetainedAt = time.Now().UTC()
	entries := append([]runner.RetainedCase(nil), s.bySig[candidate.Signature.StableKey]...)
	entries = append(entries, candidate)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Score > entries[j].Score })
	if len(entries) > s.cap {
		entries = entries[:s.cap]
	}
	s.bySig[candidate.Signature.StableKey] = entries
	return s.write(candidate.Signature.StableKey, entries)
}

func (s *Store) Entries(signature string) []runner.RetainedCase {
	return append([]runner.RetainedCase(nil), s.bySig[signature]...)
}

func (s *Store) write(signature string, entries []runner.RetainedCase) error {
	dir := filepath.Join(s.root, signature)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Case.CaseID+".json")
		blob, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, blob, 0o644); err != nil {
			return err
		}
	}
	return nil
}
