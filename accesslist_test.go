package txfuzz

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestCreateAccessListUsesLegacyGasFieldShape(t *testing.T) {
	to := common.HexToAddress("0x1234")
	tx := types.NewTx(&types.AccessListTx{ChainID: big.NewInt(1), Nonce: 1, To: &to, Gas: 21000, GasPrice: big.NewInt(7), Value: big.NewInt(0)})
	msg := accessListCallMsg(tx, common.HexToAddress("0xabcd"))
	if msg.GasPrice == nil || msg.GasPrice.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("expected gas price 7, got %v", msg.GasPrice)
	}
	if msg.GasFeeCap != nil || msg.GasTipCap != nil {
		t.Fatalf("expected legacy gas shape, got feeCap=%v tipCap=%v", msg.GasFeeCap, msg.GasTipCap)
	}
}

func TestCreateAccessListUses1559GasFieldShape(t *testing.T) {
	to := common.HexToAddress("0x1234")
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), Nonce: 1, To: &to, Gas: 21000, GasTipCap: big.NewInt(2), GasFeeCap: big.NewInt(9), Value: big.NewInt(0)})
	msg := accessListCallMsg(tx, common.HexToAddress("0xabcd"))
	if msg.GasPrice != nil {
		t.Fatalf("expected nil gas price, got %v", msg.GasPrice)
	}
	if msg.GasFeeCap == nil || msg.GasFeeCap.Cmp(big.NewInt(9)) != 0 {
		t.Fatalf("expected fee cap 9, got %v", msg.GasFeeCap)
	}
	if msg.GasTipCap == nil || msg.GasTipCap.Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("expected tip cap 2, got %v", msg.GasTipCap)
	}
}
