package runner

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestConfirmTrackerTracksSentAsAwaitingReceipt(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	tracker := NewConfirmTracker(2, 2*time.Second)
	hash := common.HexToHash("0x1")

	tx, transition := tracker.TrackSent(TrackedTx{
		CaseID: "case-1",
		LaneID: "lane-a",
		Hash:   hash,
		At:     now,
	})

	if tx.ConfirmState != ConfirmStateAwaitingReceipt {
		t.Fatalf("unexpected confirm state: %s", tx.ConfirmState)
	}
	if transition.From != ConfirmStateNotTracked || transition.To != ConfirmStateAwaitingReceipt {
		t.Fatalf("unexpected transition: %#v", transition)
	}
	if transition.Terminal {
		t.Fatalf("sent should not be terminal")
	}
}

func TestConfirmTrackerMarksSoftDeadlineBreachWithoutTerminalizing(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	tracker := NewConfirmTracker(2, 2*time.Second)
	hash := common.HexToHash("0x2")
	tracker.TrackSent(TrackedTx{CaseID: "case-2", LaneID: "lane-a", Hash: hash, At: now})

	transitions := tracker.MarkSLABreaches(now.Add(3 * time.Second))
	if len(transitions) != 1 {
		t.Fatalf("expected one sla transition, got %d", len(transitions))
	}
	if transitions[0].To != ConfirmStateSLABreachedPending {
		t.Fatalf("unexpected sla transition: %#v", transitions[0])
	}
	if transitions[0].Terminal {
		t.Fatalf("sla breach must stay non-terminal")
	}

	tx := tracker.Tracked(hash)
	if tx == nil || tx.ConfirmState != ConfirmStateSLABreachedPending {
		t.Fatalf("unexpected tracked state after sla breach: %#v", tx)
	}
}

func TestConfirmTrackerReceiptSuccessAndFailureBecomeTerminalIncludedStates(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	successHash := common.HexToHash("0x3")
	revertHash := common.HexToHash("0x4")
	tracker := NewConfirmTracker(2, 2*time.Second)
	tracker.TrackSent(TrackedTx{CaseID: "case-3", LaneID: "lane-a", Hash: successHash, At: now})
	tracker.TrackSent(TrackedTx{CaseID: "case-4", LaneID: "lane-a", Hash: revertHash, At: now})

	okStatus := uint64(1)
	revertStatus := uint64(0)
	transitions := tracker.ApplyReceiptResults(map[common.Hash]ReceiptResult{
		successHash: {Found: true, Status: &okStatus, BlockNumber: 10, GasUsed: 21_000},
		revertHash:  {Found: true, Status: &revertStatus, BlockNumber: 11, GasUsed: 22_000},
	}, now.Add(time.Second))

	if len(transitions) != 2 {
		t.Fatalf("expected two receipt transitions, got %d", len(transitions))
	}

	successTx := tracker.Tracked(successHash)
	if successTx == nil || successTx.ConfirmState != ConfirmStateIncludedSuccess || !successTx.ReceiptObserved {
		t.Fatalf("unexpected success state: %#v", successTx)
	}
	revertTx := tracker.Tracked(revertHash)
	if revertTx == nil || revertTx.ConfirmState != ConfirmStateIncludedReverted || !revertTx.ReceiptObserved {
		t.Fatalf("unexpected reverted state: %#v", revertTx)
	}
}

func TestConfirmTrackerShutdownMarksPendingAsUnresolved(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	hash := common.HexToHash("0x5")
	tracker := NewConfirmTracker(2, 2*time.Second)
	tracker.TrackSent(TrackedTx{CaseID: "case-5", LaneID: "lane-a", Hash: hash, At: now})

	transitions := tracker.FinalizeShutdown(now.Add(5 * time.Second))
	if len(transitions) != 1 {
		t.Fatalf("expected one shutdown transition, got %d", len(transitions))
	}
	if transitions[0].To != ConfirmStateUnresolvedShutdown || !transitions[0].Terminal {
		t.Fatalf("unexpected shutdown transition: %#v", transitions[0])
	}

	tx := tracker.Tracked(hash)
	if tx == nil || tx.ConfirmState != ConfirmStateUnresolvedShutdown || tx.ReceiptObserved {
		t.Fatalf("unexpected shutdown state: %#v", tx)
	}
}

func TestConfirmTrackerMarksDroppedOrReplacedAsTerminal(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	hash := common.HexToHash("0x6")
	tracker := NewConfirmTracker(2, 2*time.Second)
	tracker.TrackSent(TrackedTx{CaseID: "case-6", LaneID: "lane-a", Hash: hash, At: now})

	transitions := tracker.ApplyReceiptResults(map[common.Hash]ReceiptResult{
		hash: {Dropped: true},
	}, now.Add(time.Second))
	if len(transitions) != 1 {
		t.Fatalf("expected one dropped transition, got %d", len(transitions))
	}
	if transitions[0].To != ConfirmStateDroppedOrReplaced || !transitions[0].Terminal {
		t.Fatalf("unexpected dropped transition: %#v", transitions[0])
	}
}

func TestConfirmTrackerPendingHashesBatchesByConfiguredSize(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	tracker := NewConfirmTracker(2, 2*time.Second)
	hashes := []common.Hash{
		common.HexToHash("0x10"),
		common.HexToHash("0x11"),
		common.HexToHash("0x12"),
	}
	for i, hash := range hashes {
		tracker.TrackSent(TrackedTx{
			CaseID: "case-batch",
			LaneID: "lane-a",
			Hash:   hash,
			At:     now.Add(time.Duration(i) * time.Millisecond),
		})
	}

	batches := tracker.PendingHashes()
	if len(batches) != 2 {
		t.Fatalf("expected two batches, got %d", len(batches))
	}
	if len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("unexpected batch sizes: %#v", batches)
	}
	if batches[0][0] != hashes[0] || batches[0][1] != hashes[1] || batches[1][0] != hashes[2] {
		t.Fatalf("unexpected batch ordering: %#v", batches)
	}
}
