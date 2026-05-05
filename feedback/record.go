package feedback

import "time"

type ProcessSignal struct {
	ExitObserved bool     `json:"exit_observed"`
	ExitCode     *int     `json:"exit_code,omitempty"`
	StderrHints  []string `json:"stderr_hints,omitempty"`
}

type Record struct {
	CaseID               string        `json:"case_id"`
	RPCLabel             string        `json:"rpc_label"`
	SubmitStartedAt      time.Time     `json:"submit_started_at"`
	SubmitLatencyMS      int64         `json:"submit_latency_ms"`
	SendStatus           string        `json:"send_status"`
	LaneID               string        `json:"lane_id,omitempty"`
	SendState            string        `json:"send_state,omitempty"`
	ConfirmState         string        `json:"confirm_state,omitempty"`
	RPCErrorClass        string        `json:"rpc_error_class,omitempty"`
	RPCErrorMessage      string        `json:"rpc_error_message,omitempty"`
	ReceiptObserved      bool          `json:"receipt_observed"`
	ReceiptStatus        *uint64       `json:"receipt_status,omitempty"`
	ReceiptGasUsed       *uint64       `json:"receipt_gas_used,omitempty"`
	ReceiptBlockNumber   *uint64       `json:"receipt_block_number,omitempty"`
	InclusionLatencyMS   *int64        `json:"inclusion_latency_ms,omitempty"`
	LastObservedAt       *time.Time    `json:"last_observed_at,omitempty"`
	ExecutionBucket      string        `json:"execution_bucket,omitempty"`
	AnomalyFlags         []string      `json:"anomaly_flags,omitempty"`
	ReconciliationCount  int           `json:"reconciliation_count,omitempty"`
	SoftDeadlineBreached bool          `json:"soft_deadline_breached,omitempty"`
	FinalizedAt          *time.Time    `json:"finalized_at,omitempty"`
	ProcessSignals       ProcessSignal `json:"process_signals,omitempty"`
}
