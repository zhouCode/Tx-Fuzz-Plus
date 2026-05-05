package spammer

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestEnsureIntrinsicGasFloorRaisesContractCreationGas(t *testing.T) {
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   common.Big1,
		Nonce:     1,
		GasTipCap: common.Big1,
		GasFeeCap: common.Big2,
		Gas:       26_432,
		Data:      make([]byte, 128),
	})

	if err := ensureIntrinsicGasFloor(tx); err != nil {
		t.Fatalf("ensure intrinsic gas floor: %v", err)
	}

	intrinsic, err := core.IntrinsicGas(tx.Data(), tx.AccessList(), tx.SetCodeAuthorizations(), tx.To() == nil, true, true, true)
	if err != nil {
		t.Fatalf("intrinsic gas: %v", err)
	}
	if tx.Gas() != intrinsic {
		t.Fatalf("expected gas raised to intrinsic floor %d, got %d", intrinsic, tx.Gas())
	}
}
