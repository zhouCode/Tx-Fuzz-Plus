package runner

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

type stubPendingNonceSource struct {
	nonces map[common.Address]uint64
	calls  map[common.Address]int
}

func (s *stubPendingNonceSource) PendingNonceAt(_ context.Context, address common.Address) (uint64, error) {
	if s.calls == nil {
		s.calls = make(map[common.Address]int)
	}
	s.calls[address]++
	return s.nonces[address], nil
}

func TestAddressNonceManagerLeasesMonotonicallyPerAddress(t *testing.T) {
	addr := common.HexToAddress("0x1001")
	source := &stubPendingNonceSource{nonces: map[common.Address]uint64{addr: 7}}
	manager := NewAddressNonceManager(source)

	first, err := manager.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("first lease: %v", err)
	}
	second, err := manager.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("second lease: %v", err)
	}

	if first != 7 || second != 8 {
		t.Fatalf("expected monotonic leases 7 then 8, got %d then %d", first, second)
	}
	if source.calls[addr] != 1 {
		t.Fatalf("expected one source call for %s, got %d", addr.Hex(), source.calls[addr])
	}
}

func TestAddressNonceManagerUsesIndependentAddressDomains(t *testing.T) {
	addrA := common.HexToAddress("0x1001")
	addrB := common.HexToAddress("0x1002")
	source := &stubPendingNonceSource{nonces: map[common.Address]uint64{
		addrA: 3,
		addrB: 11,
	}}
	manager := NewAddressNonceManager(source)

	leaseA1, err := manager.Lease(context.Background(), addrA)
	if err != nil {
		t.Fatalf("lease a1: %v", err)
	}
	leaseB1, err := manager.Lease(context.Background(), addrB)
	if err != nil {
		t.Fatalf("lease b1: %v", err)
	}
	leaseA2, err := manager.Lease(context.Background(), addrA)
	if err != nil {
		t.Fatalf("lease a2: %v", err)
	}
	leaseB2, err := manager.Lease(context.Background(), addrB)
	if err != nil {
		t.Fatalf("lease b2: %v", err)
	}

	if leaseA1 != 3 || leaseA2 != 4 {
		t.Fatalf("unexpected leases for address A: %d then %d", leaseA1, leaseA2)
	}
	if leaseB1 != 11 || leaseB2 != 12 {
		t.Fatalf("unexpected leases for address B: %d then %d", leaseB1, leaseB2)
	}
}

func TestLaneReusesSharedAddressLeaseWithinCase(t *testing.T) {
	addr := common.HexToAddress("0x1001")
	source := &stubPendingNonceSource{nonces: map[common.Address]uint64{addr: 9}}
	orchestrator := NewOrchestrator(source)

	lane := orchestrator.NewLane()
	txNonce, err := lane.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("tx lease: %v", err)
	}
	authNonce, err := lane.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("auth lease: %v", err)
	}
	if txNonce != authNonce {
		t.Fatalf("expected shared-address lane reuse, got tx nonce %d and auth nonce %d", txNonce, authNonce)
	}

	nextLane := orchestrator.NewLane()
	nextNonce, err := nextLane.Lease(context.Background(), addr)
	if err != nil {
		t.Fatalf("next lane lease: %v", err)
	}
	if nextNonce != txNonce+1 {
		t.Fatalf("expected next lane to advance to %d, got %d", txNonce+1, nextNonce)
	}
	if source.calls[addr] != 1 {
		t.Fatalf("expected one source call for %s, got %d", addr.Hex(), source.calls[addr])
	}
}
