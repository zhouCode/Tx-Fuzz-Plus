package runner

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type PendingNonceSource interface {
	PendingNonceAt(context.Context, common.Address) (uint64, error)
}

type AddressNonceManager struct {
	source PendingNonceSource

	mu     sync.Mutex
	next   map[common.Address]uint64
	loaded map[common.Address]bool
}

func NewAddressNonceManager(source PendingNonceSource) *AddressNonceManager {
	return &AddressNonceManager{
		source: source,
		next:   make(map[common.Address]uint64),
		loaded: make(map[common.Address]bool),
	}
}

func (m *AddressNonceManager) Lease(ctx context.Context, address common.Address) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.loaded[address] {
		nonce, err := m.source.PendingNonceAt(ctx, address)
		if err != nil {
			return 0, err
		}
		m.next[address] = nonce
		m.loaded[address] = true
	}
	nonce := m.next[address]
	m.next[address]++
	return nonce, nil
}
