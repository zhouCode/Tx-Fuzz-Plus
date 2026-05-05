package runner

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestOrchestratorNewLaneSharesManagerStateAcrossLanes(t *testing.T) {
	addr := common.HexToAddress("0x2001")
	source := &stubPendingNonceSource{nonces: map[common.Address]uint64{addr: 5}}
	orchestrator := NewOrchestrator(source)

	laneA := orchestrator.NewLane()
	nonceA, err := laneA.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("lane A lease: %v", err)
	}

	laneB := orchestrator.NewLane()
	nonceB, err := laneB.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("lane B lease: %v", err)
	}

	if nonceA != 5 || nonceB != 6 {
		t.Fatalf("expected shared manager progression 5 then 6, got %d then %d", nonceA, nonceB)
	}
	if source.calls[addr] != 1 {
		t.Fatalf("expected one source call for %s, got %d", addr.Hex(), source.calls[addr])
	}
}

func TestOrchestratorNewLaneCreatesIndependentLaneCaches(t *testing.T) {
	addr := common.HexToAddress("0x2002")
	source := &stubPendingNonceSource{nonces: map[common.Address]uint64{addr: 12}}
	orchestrator := NewOrchestrator(source)

	laneA := orchestrator.NewLane()
	firstA, err := laneA.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("lane A first lease: %v", err)
	}
	reusedA, err := laneA.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("lane A reused lease: %v", err)
	}
	if firstA != reusedA {
		t.Fatalf("expected lane A to reuse nonce %d, got %d", firstA, reusedA)
	}

	laneB := orchestrator.NewLane()
	firstB, err := laneB.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("lane B first lease: %v", err)
	}
	if firstB != firstA+1 {
		t.Fatalf("expected lane B to observe next shared nonce %d, got %d", firstA+1, firstB)
	}
}
