package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEventLogAppendsJSONLRecordsUnderCanonicalEventsPath(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1700000000, 0).UTC()
	log := NewEventLog(root)

	first := EventRecord{
		At:           now,
		CaseID:       "case-1",
		LaneID:       "lane-a",
		EventType:    EventTypeNonceReserved,
		SendState:    "nonce_reserved",
		ConfirmState: ConfirmStateNotTracked,
	}
	second := EventRecord{
		At:           now.Add(time.Second),
		CaseID:       "case-1",
		LaneID:       "lane-a",
		EventType:    EventTypeSent,
		SendState:    "sent",
		ConfirmState: ConfirmStateAwaitingReceipt,
	}

	if err := log.Append(first); err != nil {
		t.Fatalf("append first record: %v", err)
	}
	if err := log.Append(second); err != nil {
		t.Fatalf("append second record: %v", err)
	}

	path := filepath.Join(root, "events", "events.jsonl")
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}

	lines := splitNonEmptyLines(string(blob))
	if len(lines) != 2 {
		t.Fatalf("expected 2 jsonl lines, got %d", len(lines))
	}

	var gotFirst, gotSecond EventRecord
	if err := json.Unmarshal([]byte(lines[0]), &gotFirst); err != nil {
		t.Fatalf("unmarshal first line: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &gotSecond); err != nil {
		t.Fatalf("unmarshal second line: %v", err)
	}
	if gotFirst.EventType != EventTypeNonceReserved || gotSecond.EventType != EventTypeSent {
		t.Fatalf("unexpected event types: %#v / %#v", gotFirst, gotSecond)
	}
	if !gotFirst.At.Before(gotSecond.At) {
		t.Fatalf("expected monotonic event ordering, got %#v then %#v", gotFirst.At, gotSecond.At)
	}
}

func TestEventLogSupportsTerminalConfirmationEvidence(t *testing.T) {
	root := t.TempDir()
	log := NewEventLog(root)
	now := time.Unix(1700000010, 0).UTC()

	record := EventRecord{
		At:           now,
		CaseID:       "case-2",
		LaneID:       "lane-a",
		EventType:    EventTypeUnresolvedShutdown,
		SendState:    "sent",
		ConfirmState: ConfirmStateUnresolvedShutdown,
		TxHash:       "0xabc",
	}
	if err := log.Append(record); err != nil {
		t.Fatalf("append terminal record: %v", err)
	}

	records, err := ReadEventLog(filepath.Join(root, "events", "events.jsonl"))
	if err != nil {
		t.Fatalf("read event records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}
	if records[0].ConfirmState != ConfirmStateUnresolvedShutdown || records[0].EventType != EventTypeUnresolvedShutdown {
		t.Fatalf("unexpected terminal record: %#v", records[0])
	}
}
