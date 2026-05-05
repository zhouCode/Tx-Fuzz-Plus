package runner

import "time"

type LaneReport struct {
	LaneID             string         `json:"lane_id,omitempty"`
	RPCLabel           string         `json:"rpc_label,omitempty"`
	ELClient           string         `json:"el_client,omitempty"`
	TotalCases         int            `json:"total_cases,omitempty"`
	SentCases          int            `json:"sent_cases,omitempty"`
	RetainedCases      int            `json:"retained_cases,omitempty"`
	DuplicateCases     int            `json:"duplicate_cases,omitempty"`
	MaxInFlight        int            `json:"max_in_flight,omitempty"`
	SendStateCounts    map[string]int `json:"send_state_counts,omitempty"`
	ConfirmStateCounts map[string]int `json:"confirm_state_counts,omitempty"`
	AnomalyCounts      map[string]int `json:"anomaly_counts,omitempty"`
}

type ConfirmStats struct {
	ByState             map[string]int `json:"by_state,omitempty"`
	TerminalCases       int            `json:"terminal_cases,omitempty"`
	UnresolvedShutdowns int            `json:"unresolved_shutdowns,omitempty"`
}

type ThroughputSummary struct {
	SubmitDurationMS int64   `json:"submit_duration_ms,omitempty"`
	ConfirmDrainMS   int64   `json:"confirm_drain_ms,omitempty"`
	TotalDurationMS  int64   `json:"total_duration_ms,omitempty"`
	CasesPerSecond   float64 `json:"cases_per_second,omitempty"`
}

type ReportV2 struct {
	CampaignID        string             `json:"campaign_id"`
	TxFamily          string             `json:"tx_family"`
	TotalCases        int                `json:"total_cases"`
	SentCases         int                `json:"sent_cases"`
	RetainedCases     int                `json:"retained_cases"`
	DuplicateCases    int                `json:"duplicate_cases"`
	GeneratedAt       time.Time          `json:"generated_at"`
	SendArchitecture  string             `json:"send_architecture,omitempty"`
	LaneStats         []LaneReport       `json:"lane_stats,omitempty"`
	ConfirmStats      *ConfirmStats      `json:"confirm_stats,omitempty"`
	ThroughputSummary *ThroughputSummary `json:"throughput_summary,omitempty"`
	AnomalyBreakdown  map[string]int     `json:"anomaly_breakdown,omitempty"`
}

func WriteReportV2(path string, report ReportV2) error {
	if report.GeneratedAt.IsZero() {
		report.GeneratedAt = time.Now().UTC()
	}
	return writeJSON(path, report)
}
