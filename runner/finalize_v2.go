package runner

import (
	"context"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
)

func drainOneFinalizedCase(ctx context.Context, pending []pendingCase, sink Sink, stats *Stats, report *ReportV2, limiter *InFlightLimiter, opts V2Options, receiptAwaiter ReceiptAwaiter) ([]pendingCase, error) {
	item := pending[0]
	rec, err := item.pending.Await(ctx)
	if err != nil {
		return pending, err
	}
	if rec.CaseID == "" {
		rec.CaseID = item.record.CaseID
	}
	if rec.LaneID == "" {
		rec.LaneID = item.record.LaneID
	}
	if rec.SendState == "" {
		rec.SendState = rec.SendStatus
	}
	if rec.SendStatus == "sent" {
		sentAt := time.Now().UTC()
		rec.SendState = "sent"
		if opts.EventLog != nil {
			_ = opts.EventLog.Append(EventRecord{
				At:        sentAt,
				CaseID:    item.record.CaseID,
				LaneID:    rec.LaneID,
				TxHash:    item.record.SignedTxHash,
				EventType: EventTypeSent,
				SendState: rec.SendState,
			})
		}
		receiptCtx, cancel := context.WithTimeout(ctx, opts.ReceiptTimeout)
		receiptRec, awaitErr := receiptAwaiter.AwaitReceipt(receiptCtx, item.tx)
		cancel()
		if awaitErr != nil {
			return pending, awaitErr
		}
		mergeReceiptRecord(&rec, receiptRec)
		slaStart := rec.SubmitStartedAt
		if slaStart.IsZero() {
			slaStart = sentAt
		}
		if item.record.RunStartedAt.Before(slaStart) {
			slaStart = item.record.RunStartedAt
		}
		applyConfirmBudgetState(&rec, item.record.RunStartedAt, slaStart, opts)
	} else {
		rec.ConfirmState = ""
	}

	finalizedAt := time.Now().UTC()
	rec.FinalizedAt = &finalizedAt
	finalizeConfirmState(&rec)
	if opts.EventLog != nil {
		appendFinalizeEvent(opts.EventLog, item.record, rec, finalizedAt)
	}
	if err := acceptOutcome(ctx, item.record, rec, sink, stats); err != nil {
		return pending, err
	}
	if limiter != nil {
		limiter.Release(opts.LaneID)
	}
	updateV2Report(report, rec)
	return pending[1:], nil
}

func mergeReceiptRecord(dst *feedback.Record, src feedback.Record) {
	if src.ReceiptObserved {
		dst.ReceiptObserved = true
		dst.ReceiptStatus = src.ReceiptStatus
		dst.ReceiptGasUsed = src.ReceiptGasUsed
		dst.ReceiptBlockNumber = src.ReceiptBlockNumber
		dst.InclusionLatencyMS = src.InclusionLatencyMS
		dst.ExecutionBucket = src.ExecutionBucket
	}
	if len(src.AnomalyFlags) > 0 {
		dst.AnomalyFlags = append(dst.AnomalyFlags, src.AnomalyFlags...)
	}
	if len(src.ProcessSignals.StderrHints) > 0 {
		dst.ProcessSignals.StderrHints = append(dst.ProcessSignals.StderrHints, src.ProcessSignals.StderrHints...)
	}
}

func applyConfirmBudgetState(rec *feedback.Record, startedAt, sentAt time.Time, opts V2Options) {
	if rec.ReceiptObserved {
		return
	}
	now := time.Now().UTC()
	if opts.ConfirmSLA > 0 && now.Sub(sentAt) >= opts.ConfirmSLA {
		rec.SoftDeadlineBreached = true
		rec.ConfirmState = string(ConfirmStateSLABreachedPending)
		if !hasFlag(rec.AnomalyFlags, string(ConfirmStateSLABreachedPending)) {
			rec.AnomalyFlags = append(rec.AnomalyFlags, string(ConfirmStateSLABreachedPending))
		}
	}
	if hasFlag(rec.AnomalyFlags, "receipt_timeout") && opts.ConfirmDrain > 0 && now.Sub(startedAt) >= opts.ConfirmDrain {
		rec.ConfirmState = string(ConfirmStateUnresolvedShutdown)
		if !hasFlag(rec.AnomalyFlags, string(ConfirmStateUnresolvedShutdown)) {
			rec.AnomalyFlags = append(rec.AnomalyFlags, string(ConfirmStateUnresolvedShutdown))
		}
	}
}

func finalizeConfirmState(rec *feedback.Record) {
	if rec.SendStatus != "sent" {
		rec.SendState = rec.SendStatus
		rec.ConfirmState = ""
		return
	}
	if rec.SendState == "" {
		rec.SendState = "sent"
	}
	if rec.ConfirmState != "" {
		return
	}
	switch {
	case rec.ReceiptObserved && rec.ReceiptStatus != nil && *rec.ReceiptStatus == 0:
		rec.ConfirmState = string(ConfirmStateIncludedReverted)
	case rec.ReceiptObserved:
		rec.ConfirmState = string(ConfirmStateIncludedSuccess)
	case hasFlag(rec.AnomalyFlags, string(ConfirmStateUnresolvedShutdown)):
		rec.ConfirmState = string(ConfirmStateUnresolvedShutdown)
	case hasFlag(rec.AnomalyFlags, "receipt_timeout"):
		rec.ConfirmState = string(ConfirmStateAwaitingReceipt)
	default:
		rec.ConfirmState = string(ConfirmStateAwaitingReceipt)
	}
}

func appendFinalizeEvent(eventLog *EventLog, record TestcaseRecord, rec feedback.Record, at time.Time) {
	switch {
	case rec.SendStatus != "sent":
		_ = eventLog.Append(EventRecord{
			At:        at,
			CaseID:    record.CaseID,
			LaneID:    rec.LaneID,
			TxHash:    record.SignedTxHash,
			EventType: EventTypeSendRejected,
			SendState: rec.SendState,
		})
	case rec.ReceiptObserved:
		_ = eventLog.Append(EventRecord{
			At:           at,
			CaseID:       record.CaseID,
			LaneID:       rec.LaneID,
			TxHash:       record.SignedTxHash,
			EventType:    EventTypeReceiptSeen,
			SendState:    rec.SendState,
			ConfirmState: ConfirmState(rec.ConfirmState),
		})
	case rec.ConfirmState == string(ConfirmStateUnresolvedShutdown):
		_ = eventLog.Append(EventRecord{
			At:           at,
			CaseID:       record.CaseID,
			LaneID:       rec.LaneID,
			TxHash:       record.SignedTxHash,
			EventType:    EventTypeUnresolvedShutdown,
			SendState:    rec.SendState,
			ConfirmState: ConfirmState(rec.ConfirmState),
		})
	case rec.ConfirmState == string(ConfirmStateSLABreachedPending):
		_ = eventLog.Append(EventRecord{
			At:           at,
			CaseID:       record.CaseID,
			LaneID:       rec.LaneID,
			TxHash:       record.SignedTxHash,
			EventType:    EventTypeSLABreach,
			SendState:    rec.SendState,
			ConfirmState: ConfirmState(rec.ConfirmState),
		})
	}
}

func updateV2Report(report *ReportV2, rec feedback.Record) {
	if report == nil || len(report.LaneStats) == 0 || report.ConfirmStats == nil {
		return
	}
	lane := &report.LaneStats[0]
	lane.SendStateCounts[rec.SendState]++
	if rec.ConfirmState != "" {
		lane.ConfirmStateCounts[rec.ConfirmState]++
		report.ConfirmStats.ByState[rec.ConfirmState]++
	}
	for _, flag := range rec.AnomalyFlags {
		lane.AnomalyCounts[flag]++
		report.AnomalyBreakdown[flag]++
	}
	if isTerminalConfirmState(rec.ConfirmState) {
		report.ConfirmStats.TerminalCases++
	}
	if rec.ConfirmState == string(ConfirmStateUnresolvedShutdown) {
		report.ConfirmStats.UnresolvedShutdowns++
	}
}

func isTerminalConfirmState(state string) bool {
	switch ConfirmState(state) {
	case ConfirmStateIncludedSuccess, ConfirmStateIncludedReverted, ConfirmStateDroppedOrReplaced, ConfirmStateUnresolvedShutdown:
		return true
	default:
		return false
	}
}

func hasFlag(flags []string, want string) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}
