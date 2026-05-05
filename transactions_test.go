package txfuzz

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestApplyIntrinsicGasFloorRaisesContractCreationBeforeAccessList(t *testing.T) {
	raw := types.NewTx(&types.DynamicFeeTx{
		Nonce:     1,
		GasTipCap: big1(),
		GasFeeCap: big2(),
		Gas:       26_432,
		Data:      make([]byte, 128),
	})
	tx, err := applyIntrinsicGasFloor(raw)
	if err != nil {
		t.Fatalf("apply intrinsic gas floor: %v", err)
	}
	intrinsic, err := core.IntrinsicGas(tx.Data(), tx.AccessList(), tx.SetCodeAuthorizations(), tx.To() == nil, true, true, true)
	if err != nil {
		t.Fatalf("intrinsic gas: %v", err)
	}
	if tx.Gas() != intrinsic {
		t.Fatalf("expected intrinsic gas floor %d, got %d", intrinsic, tx.Gas())
	}
}

func big1() *big.Int { return big.NewInt(1) }
func big2() *big.Int { return big.NewInt(2) }
