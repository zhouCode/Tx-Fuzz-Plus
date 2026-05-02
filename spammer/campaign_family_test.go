package spammer

import (
	"testing"

	"github.com/MariusVanDerWijden/tx-fuzz/runner"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestFamilyMetadataHelpers(t *testing.T) {
	blobTx := types.NewTx(&types.BlobTx{})
	blobRecord := runner.TestcaseRecord{FeeFields: map[string]string{}}
	applyFamilyMetadata("blob", blobRecord.FeeFields, blobTx, &blobRecord, nil)
	if blobRecord.BlobCount < 0 {
		t.Fatalf("unexpected blob count: %d", blobRecord.BlobCount)
	}

	auths := []types.SetCodeAuthorization{{}}
	authTx := types.NewTx(&types.SetCodeTx{AuthList: auths})
	authRecord := runner.TestcaseRecord{FeeFields: map[string]string{}}
	applyFamilyMetadata("pectra", authRecord.FeeFields, authTx, &authRecord, auths)
	if authRecord.AuthorizationCount != 1 {
		t.Fatalf("expected auth count 1, got %d", authRecord.AuthorizationCount)
	}
}
