package spammer

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/tx-fuzz/runner"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestBuildCampaignTxPectraSharesNonceDomainForSameAddress(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	source := &stubCampaignPendingNonceSource{nonces: map[common.Address]uint64{sender: 13}}
	lane := runner.NewOrchestrator(source).NewLane()
	senderNonce, err := lane.Lease(context.Background(), sender)
	if err != nil {
		t.Fatalf("lease sender nonce: %v", err)
	}

	tx, auths, _, family, err := buildCampaignTx(
		context.Background(),
		&Config{keys: []*ecdsa.PrivateKey{key}},
		nil,
		big.NewInt(1),
		"pectra",
		key,
		sender,
		senderNonce,
		filler.NewFiller([]byte{0x1, 0x2, 0x3}),
		1,
		lane,
	)
	if err != nil {
		t.Fatalf("build pectra tx: %v", err)
	}
	if family != "pectra" {
		t.Fatalf("expected pectra family, got %s", family)
	}
	if tx.Nonce() != senderNonce {
		t.Fatalf("expected tx nonce %d, got %d", senderNonce, tx.Nonce())
	}
	if len(auths) != 1 {
		t.Fatalf("expected one authorization, got %d", len(auths))
	}
	if auths[0].Nonce != senderNonce {
		t.Fatalf("expected auth nonce %d to share sender domain, got %d", senderNonce, auths[0].Nonce)
	}
}

type stubCampaignPendingNonceSource struct {
	nonces map[common.Address]uint64
}

func (s *stubCampaignPendingNonceSource) PendingNonceAt(_ context.Context, address common.Address) (uint64, error) {
	return s.nonces[address], nil
}
