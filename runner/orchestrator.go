package runner

import "github.com/ethereum/go-ethereum/common"

type Orchestrator struct {
	manager *AddressNonceManager
}

func NewOrchestrator(source PendingNonceSource) *Orchestrator {
	return &Orchestrator{manager: NewAddressNonceManager(source)}
}

func (o *Orchestrator) NewLane() *Lane {
	return &Lane{
		manager: o.manager,
		leases:  make(map[common.Address]uint64),
	}
}
