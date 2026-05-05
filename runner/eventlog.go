package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EventType string

const (
	EventTypeNonceReserved      EventType = "nonce_reserved"
	EventTypeBuilt              EventType = "built"
	EventTypeSent               EventType = "sent"
	EventTypeSendRejected       EventType = "send_rejected"
	EventTypeReceiptSeen        EventType = "receipt_seen"
	EventTypeSLABreach          EventType = "sla_breach"
	EventTypeUnresolvedShutdown EventType = "unresolved_shutdown"
)

type EventRecord struct {
	At           time.Time    `json:"at"`
	CaseID       string       `json:"case_id,omitempty"`
	LaneID       string       `json:"lane_id,omitempty"`
	TxHash       string       `json:"tx_hash,omitempty"`
	EventType    EventType    `json:"event_type"`
	SendState    string       `json:"send_state,omitempty"`
	ConfirmState ConfirmState `json:"confirm_state,omitempty"`
}

type EventLog struct {
	path string
}

func NewEventLog(root string) *EventLog {
	return &EventLog{path: filepath.Join(root, "events", "events.jsonl")}
}

func (l *EventLog) Path() string {
	return l.path
}

func (l *EventLog) Append(record EventRecord) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	blob, err := json.Marshal(record)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(blob, '\n')); err != nil {
		return err
	}
	return nil
}

func ReadEventLog(path string) ([]EventRecord, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := splitNonEmptyLines(string(blob))
	records := make([]EventRecord, 0, len(lines))
	for _, line := range lines {
		var record EventRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func splitNonEmptyLines(s string) []string {
	scanner := bufio.NewScanner(bytes.NewBufferString(s))
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
