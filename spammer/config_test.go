package spammer

import (
	"testing"

	"github.com/MariusVanDerWijden/tx-fuzz/flags"
)

func TestGasLimitDefaultsWhenFlagMissing(t *testing.T) {
	gasLimit := 0
	if gasLimit <= 0 {
		gasLimit = flags.GasLimitFlag.Value
	}
	if gasLimit != flags.GasLimitFlag.Value {
		t.Fatalf("expected default gas limit %d, got %d", flags.GasLimitFlag.Value, gasLimit)
	}
}
