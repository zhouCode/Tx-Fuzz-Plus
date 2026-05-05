package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReportV2JSONRoundTripPreservesLegacyTotalsAndAdditiveSections(t *testing.T) {
	generatedAt := time.Unix(1700000100, 0).UTC()
	report := ReportV2{
		CampaignID:        "campaign-001",
		TxFamily:          "basic",
		TotalCases:        12,
		SentCases:         10,
		RetainedCases:     3,
		DuplicateCases:    1,
		GeneratedAt:       generatedAt,
		SendArchitecture:  "v2",
		LaneStats:         []LaneReport{{LaneID: "lane-a", RPCLabel: "local-geth", SentCases: 10, MaxInFlight: 4, SendStateCounts: map[string]int{"sent": 10}, ConfirmStateCounts: map[string]int{"included_success": 9, "unresolved_shutdown": 1}}},
		ConfirmStats:      &ConfirmStats{ByState: map[string]int{"included_success": 9, "unresolved_shutdown": 1}, TerminalCases: 10, UnresolvedShutdowns: 1},
		ThroughputSummary: &ThroughputSummary{SubmitDurationMS: 1000, ConfirmDrainMS: 250, TotalDurationMS: 1250, CasesPerSecond: 10},
		AnomalyBreakdown:  map[string]int{"sla_breached_pending": 1},
	}

	blob, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report v2: %v", err)
	}

	var got ReportV2
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal report v2: %v", err)
	}

	if got.CampaignID != report.CampaignID || got.TxFamily != report.TxFamily || got.TotalCases != report.TotalCases || got.SentCases != report.SentCases || got.RetainedCases != report.RetainedCases || got.DuplicateCases != report.DuplicateCases {
		t.Fatalf("legacy totals not preserved: %#v", got)
	}
	if got.SendArchitecture != "v2" || len(got.LaneStats) != 1 || got.ConfirmStats == nil || got.ThroughputSummary == nil {
		t.Fatalf("additive sections missing: %#v", got)
	}
	if got.LaneStats[0].LaneID != "lane-a" || got.LaneStats[0].MaxInFlight != 4 {
		t.Fatalf("lane stats not preserved: %#v", got.LaneStats[0])
	}
	if got.ConfirmStats.ByState["included_success"] != 9 || got.ThroughputSummary.CasesPerSecond != 10 {
		t.Fatalf("confirm/throughput summaries not preserved: %#v %#v", got.ConfirmStats, got.ThroughputSummary)
	}
}

func TestWriteReportV2WritesAdditiveReportWithoutReplacingLegacyTopLevelFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "report-v2.json")

	err := WriteReportV2(path, ReportV2{
		CampaignID:       "campaign-002",
		TxFamily:         "blob",
		TotalCases:       5,
		SentCases:        4,
		RetainedCases:    1,
		DuplicateCases:   0,
		SendArchitecture: "v2",
		LaneStats:        []LaneReport{{LaneID: "lane-a", SentCases: 4}},
	})
	if err != nil {
		t.Fatalf("write report v2: %v", err)
	}

	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report v2: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal report v2 file: %v", err)
	}

	for _, field := range []string{"campaign_id", "tx_family", "total_cases", "sent_cases", "retained_cases", "duplicate_cases", "generated_at"} {
		if _, ok := got[field]; !ok {
			t.Fatalf("missing legacy field %q in %#v", field, got)
		}
	}
	if got["send_architecture"] != "v2" {
		t.Fatalf("missing send_architecture in %#v", got)
	}
	if _, ok := got["lane_stats"]; !ok {
		t.Fatalf("missing lane_stats in %#v", got)
	}
}
