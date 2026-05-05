package runner

import (
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type ConfirmState string

const (
	ConfirmStateNotTracked         ConfirmState = "not_tracked"
	ConfirmStateAwaitingReceipt    ConfirmState = "awaiting_receipt"
	ConfirmStateSLABreachedPending ConfirmState = "sla_breached_pending"
	ConfirmStateIncludedSuccess    ConfirmState = "included_success"
	ConfirmStateIncludedReverted   ConfirmState = "included_reverted"
	ConfirmStateDroppedOrReplaced  ConfirmState = "dropped_or_replaced"
	ConfirmStateUnresolvedShutdown ConfirmState = "unresolved_shutdown"
)

type TrackedTx struct {
	CaseID           string
	LaneID           string
	Hash             common.Hash
	At               time.Time
	LastObservedAt   time.Time
	ConfirmState     ConfirmState
	ReceiptObserved  bool
	ReceiptStatus    *uint64
	ReceiptGasUsed   *uint64
	FinalBlockNumber *uint64
}

type ConfirmTransition struct {
	CaseID    string
	LaneID    string
	Hash      common.Hash
	From      ConfirmState
	To        ConfirmState
	At        time.Time
	Terminal  bool
	SoftError bool
}

type ReceiptResult struct {
	Found       bool
	Status      *uint64
	BlockNumber uint64
	GasUsed     uint64
	Dropped     bool
	Retryable   bool
}

type ConfirmTracker struct {
	batchSize int
	sla       time.Duration
	order     []common.Hash
	byHash    map[common.Hash]*TrackedTx
}

func NewConfirmTracker(batchSize int, sla time.Duration) *ConfirmTracker {
	if batchSize <= 0 {
		batchSize = 1
	}
	return &ConfirmTracker{
		batchSize: batchSize,
		sla:       sla,
		byHash:    make(map[common.Hash]*TrackedTx),
	}
}

func (t *ConfirmTracker) TrackSent(tx TrackedTx) (*TrackedTx, ConfirmTransition) {
	tx.ConfirmState = ConfirmStateAwaitingReceipt
	tx.LastObservedAt = tx.At
	copyTx := tx
	t.byHash[tx.Hash] = &copyTx
	t.order = append(t.order, tx.Hash)
	return t.byHash[tx.Hash], ConfirmTransition{
		CaseID:   tx.CaseID,
		LaneID:   tx.LaneID,
		Hash:     tx.Hash,
		From:     ConfirmStateNotTracked,
		To:       ConfirmStateAwaitingReceipt,
		At:       tx.At,
		Terminal: false,
	}
}

func (t *ConfirmTracker) Tracked(hash common.Hash) *TrackedTx {
	return t.byHash[hash]
}

func (t *ConfirmTracker) PendingHashes() [][]common.Hash {
	var pending []common.Hash
	for _, hash := range t.order {
		tx := t.byHash[hash]
		if tx == nil || isTerminal(tx.ConfirmState) {
			continue
		}
		pending = append(pending, hash)
	}
	var batches [][]common.Hash
	for len(pending) > 0 {
		n := t.batchSize
		if len(pending) < n {
			n = len(pending)
		}
		batch := append([]common.Hash(nil), pending[:n]...)
		batches = append(batches, batch)
		pending = pending[n:]
	}
	return batches
}

func (t *ConfirmTracker) MarkSLABreaches(now time.Time) []ConfirmTransition {
	var transitions []ConfirmTransition
	for _, hash := range t.order {
		tx := t.byHash[hash]
		if tx == nil || tx.ConfirmState != ConfirmStateAwaitingReceipt {
			continue
		}
		if t.sla > 0 && now.Sub(tx.At) > t.sla {
			transitions = append(transitions, t.transition(tx, ConfirmStateSLABreachedPending, now, false, true))
		}
	}
	return transitions
}

func (t *ConfirmTracker) ApplyReceiptResults(results map[common.Hash]ReceiptResult, now time.Time) []ConfirmTransition {
	keys := make([]common.Hash, 0, len(results))
	for hash := range results {
		keys = append(keys, hash)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Hex() < keys[j].Hex() })

	var transitions []ConfirmTransition
	for _, hash := range keys {
		tx := t.byHash[hash]
		if tx == nil || isTerminal(tx.ConfirmState) {
			continue
		}
		result := results[hash]
		if result.Retryable {
			transitions = append(transitions, t.transition(tx, ConfirmStateDroppedOrReplaced, now, true, false))
			continue
		}
		if !result.Found && !result.Dropped {
			continue
		}
		if result.Dropped {
			transitions = append(transitions, t.transition(tx, ConfirmStateDroppedOrReplaced, now, true, false))
			continue
		}
		tx.ReceiptObserved = true
		tx.LastObservedAt = now
		if result.Status != nil {
			status := *result.Status
			tx.ReceiptStatus = &status
		}
		gasUsed := result.GasUsed
		tx.ReceiptGasUsed = &gasUsed
		block := result.BlockNumber
		tx.FinalBlockNumber = &block
		if result.Status != nil && *result.Status == 0 {
			transitions = append(transitions, t.transition(tx, ConfirmStateIncludedReverted, now, true, false))
		} else {
			transitions = append(transitions, t.transition(tx, ConfirmStateIncludedSuccess, now, true, false))
		}
	}
	return transitions
}

func (t *ConfirmTracker) FinalizeShutdown(now time.Time) []ConfirmTransition {
	var transitions []ConfirmTransition
	for _, hash := range t.order {
		tx := t.byHash[hash]
		if tx == nil || isTerminal(tx.ConfirmState) {
			continue
		}
		transitions = append(transitions, t.transition(tx, ConfirmStateUnresolvedShutdown, now, true, false))
	}
	return transitions
}

func (t *ConfirmTracker) transition(tx *TrackedTx, to ConfirmState, at time.Time, terminal bool, soft bool) ConfirmTransition {
	from := tx.ConfirmState
	tx.ConfirmState = to
	tx.LastObservedAt = at
	return ConfirmTransition{
		CaseID:    tx.CaseID,
		LaneID:    tx.LaneID,
		Hash:      tx.Hash,
		From:      from,
		To:        to,
		At:        at,
		Terminal:  terminal,
		SoftError: soft,
	}
}

func isTerminal(state ConfirmState) bool {
	switch state {
	case ConfirmStateIncludedSuccess, ConfirmStateIncludedReverted, ConfirmStateDroppedOrReplaced, ConfirmStateUnresolvedShutdown:
		return true
	default:
		return false
	}
}
