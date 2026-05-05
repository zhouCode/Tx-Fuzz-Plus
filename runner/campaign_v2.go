package runner

import (
	"context"
	"time"
)

type V2Options struct {
	MaxInFlight      int
	ConfirmSLA       time.Duration
	ConfirmDrain     time.Duration
	ReceiptTimeout   time.Duration
	EventLog         *EventLog
	Report           *ReportV2
	LaneID           string
	RPCLabel         string
	SendArchitecture string
}

type pendingCase struct {
	record  TestcaseRecord
	tx      *Case
	pending PendingSubmission
}

func RunCampaignV2(ctx context.Context, cfg Config, builder Builder, submitter Submitter, sink Sink, opts V2Options) (Stats, error) {
	stats := Stats{CampaignID: cfg.CampaignID, TxFamily: cfg.TxFamily}
	asyncSubmitter, ok := submitter.(AsyncSubmitter)
	if !ok {
		return stats, ErrV2RequiresAsyncSubmitter
	}
	receiptAwaiter, ok := submitter.(ReceiptAwaiter)
	if !ok {
		return stats, ErrV2RequiresReceiptAwaiter
	}
	if cfg.Cases <= 0 {
		return stats, nil
	}
	if opts.MaxInFlight <= 0 {
		opts.MaxInFlight = 8
	}
	if opts.LaneID == "" {
		opts.LaneID = "lane-0"
	}
	if opts.SendArchitecture == "" {
		opts.SendArchitecture = "v2-single-lane"
	}
	if opts.ConfirmSLA <= 0 {
		opts.ConfirmSLA = 2 * time.Second
	}
	if opts.ConfirmDrain <= 0 {
		opts.ConfirmDrain = 5 * time.Second
	}
	if opts.ReceiptTimeout <= 0 || opts.ReceiptTimeout > opts.ConfirmDrain {
		opts.ReceiptTimeout = opts.ConfirmDrain
	}

	startedAt := time.Now().UTC()
	limiter := NewInFlightLimiter(opts.MaxInFlight, opts.MaxInFlight)
	report := newV2Report(cfg, opts)
	pending := make([]pendingCase, 0, cfg.Cases)
	for i := 1; i <= cfg.Cases; i++ {
		for len(pending) >= opts.MaxInFlight {
			var err error
			pending, err = drainOneFinalizedCase(ctx, pending, sink, &stats, report, limiter, opts, receiptAwaiter)
			if err != nil {
				return stats, err
			}
		}

		stats.TotalCases++
		c, err := builder.Build(ctx, i)
		if err != nil {
			return stats, err
		}
		normalizeCaseFromConfig(c, cfg, i)
		if c.Record.LaneID == "" {
			c.Record.LaneID = opts.LaneID
		}
		emitBuildEvents(opts.EventLog, c.Record)
		if !limiter.TryAcquire(opts.LaneID) {
			return stats, ErrV2LimiterRejected
		}
		p, err := asyncSubmitter.SubmitAsync(ctx, c)
		if err != nil {
			limiter.Release(opts.LaneID)
			return stats, err
		}
		pending = append(pending, pendingCase{record: c.Record, tx: c, pending: p})
		report.TotalCases++
		report.LaneStats[0].TotalCases++
		report.LaneStats[0].MaxInFlight = limiter.Snapshot().MaxLaneObserved[opts.LaneID]
	}
	submitDoneAt := time.Now().UTC()
	drainCtx, cancel := context.WithTimeout(ctx, opts.ConfirmDrain)
	defer cancel()
	for len(pending) > 0 {
		var err error
		pending, err = drainOneFinalizedCase(drainCtx, pending, sink, &stats, report, limiter, opts, receiptAwaiter)
		if err != nil {
			return stats, err
		}
	}
	finishedAt := time.Now().UTC()

	report.CampaignID = stats.CampaignID
	report.TxFamily = stats.TxFamily
	report.TotalCases = stats.TotalCases
	report.SentCases = stats.SentCases
	report.RetainedCases = stats.RetainedCases
	report.DuplicateCases = stats.DuplicateCases
	report.LaneStats[0].SentCases = stats.SentCases
	report.LaneStats[0].RetainedCases = stats.RetainedCases
	report.LaneStats[0].DuplicateCases = stats.DuplicateCases
	report.GeneratedAt = finishedAt
	report.ThroughputSummary = &ThroughputSummary{
		SubmitDurationMS: submitDoneAt.Sub(startedAt).Milliseconds(),
		ConfirmDrainMS:   finishedAt.Sub(submitDoneAt).Milliseconds(),
		TotalDurationMS:  finishedAt.Sub(startedAt).Milliseconds(),
	}
	if report.ThroughputSummary.SubmitDurationMS > 0 {
		report.ThroughputSummary.CasesPerSecond = (float64(stats.SentCases) * 1000) / float64(report.ThroughputSummary.SubmitDurationMS)
	}
	if opts.Report != nil {
		*opts.Report = *report
	}
	return stats, nil
}

var (
	ErrV2RequiresAsyncSubmitter = v2Error("v2 campaign runner requires async submitter")
	ErrV2RequiresReceiptAwaiter = v2Error("v2 campaign runner requires receipt awaiter")
	ErrV2LimiterRejected        = v2Error("v2 in-flight limiter rejected submission unexpectedly")
)

type v2Error string

func (e v2Error) Error() string { return string(e) }

func newV2Report(cfg Config, opts V2Options) *ReportV2 {
	return &ReportV2{
		CampaignID:       cfg.CampaignID,
		TxFamily:         cfg.TxFamily,
		SendArchitecture: opts.SendArchitecture,
		LaneStats: []LaneReport{{
			LaneID:             opts.LaneID,
			RPCLabel:           opts.RPCLabel,
			SendStateCounts:    make(map[string]int),
			ConfirmStateCounts: make(map[string]int),
			AnomalyCounts:      make(map[string]int),
		}},
		ConfirmStats:     &ConfirmStats{ByState: make(map[string]int)},
		AnomalyBreakdown: make(map[string]int),
	}
}

func emitBuildEvents(log *EventLog, record TestcaseRecord) {
	if log == nil {
		return
	}
	now := time.Now().UTC()
	_ = log.Append(EventRecord{
		At:        now,
		CaseID:    record.CaseID,
		LaneID:    record.LaneID,
		TxHash:    record.SignedTxHash,
		EventType: EventTypeNonceReserved,
	})
	_ = log.Append(EventRecord{
		At:        now,
		CaseID:    record.CaseID,
		LaneID:    record.LaneID,
		TxHash:    record.SignedTxHash,
		EventType: EventTypeBuilt,
	})
}
