package feedback

import "time"

type ProcessSignal struct {
	ExitObserved bool     `json:"exit_observed"`
	ExitCode     *int     `json:"exit_code,omitempty"`
	StderrHints  []string `json:"stderr_hints,omitempty"`
}

type Record struct {
	CaseID             string        `json:"case_id"`
	RPCLabel           string        `json:"rpc_label"`
	SubmitStartedAt    time.Time     `json:"submit_started_at"`
	SubmitLatencyMS    int64         `json:"submit_latency_ms"`
	SendStatus         string        `json:"send_status"`
	RPCErrorClass      string        `json:"rpc_error_class,omitempty"`
	RPCErrorMessage    string        `json:"rpc_error_message,omitempty"`
	ReceiptObserved    bool          `json:"receipt_observed"`
	ReceiptStatus      *uint64       `json:"receipt_status,omitempty"`
	ReceiptGasUsed     *uint64       `json:"receipt_gas_used,omitempty"`
	InclusionLatencyMS *int64        `json:"inclusion_latency_ms,omitempty"`
	ExecutionBucket    string        `json:"execution_bucket,omitempty"`
	AnomalyFlags       []string      `json:"anomaly_flags,omitempty"`
	ProcessSignals     ProcessSignal `json:"process_signals,omitempty"`
}
