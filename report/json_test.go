package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONCreatesParentDirsAndPersistsPayload(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "report.json")
	value := map[string]any{
		"campaign_id": "campaign-1",
		"sent_cases":  3,
	}

	if err := WriteJSON(path, value); err != nil {
		t.Fatalf("write json: %v", err)
	}

	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json file: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal json file: %v", err)
	}
	if got["campaign_id"] != "campaign-1" {
		t.Fatalf("unexpected payload: %#v", got)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("expected parent dir: %v", err)
	}
}
