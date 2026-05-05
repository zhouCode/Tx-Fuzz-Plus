package runner

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
)

type Lane struct {
	manager *AddressNonceManager
	leases  map[common.Address]uint64
}

func (l *Lane) Lease(ctx context.Context, address common.Address) (uint64, error) {
	if nonce, ok := l.leases[address]; ok {
		return nonce, nil
	}
	nonce, err := l.manager.Lease(ctx, address)
	if err != nil {
		return 0, err
	}
	l.leases[address] = nonce
	return nonce, nil
}
