package spammer

import (
	"encoding/json"
	stdflag "flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MariusVanDerWijden/tx-fuzz/flags"
	"github.com/urfave/cli/v2"
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

func newSpamTestContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()
	fs := stdflag.NewFlagSet("spam-test", stdflag.ContinueOnError)
	for _, f := range flags.SpamFlags {
		if err := f.Apply(fs); err != nil {
			t.Fatalf("apply flag %v: %v", f.Names(), err)
		}
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse args: %v", err)
	}
	return cli.NewContext(cli.NewApp(), fs, nil)
}

func TestNewConfigFromContextCarriesResolvedEndpointMetadata(t *testing.T) {
	dir := t.TempDir()

	accountsPath := filepath.Join(dir, "accounts.json")
	accountsBlob, err := json.Marshal([]AccountEntry{{
		Address:    "0x8943545177806ED17B9F23F0a21ee5948eCaa776",
		PrivateKey: "bcdf20249abf0ed6d944c0288fad489e33f66b3960d9e6229c1cd214ed3bbe31",
	}})
	if err != nil {
		t.Fatalf("marshal accounts: %v", err)
	}
	if err := os.WriteFile(accountsPath, accountsBlob, 0o644); err != nil {
		t.Fatalf("write accounts file: %v", err)
	}

	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
	}))
	defer rpcServer.Close()

	endpointsPath := filepath.Join(dir, "endpoints.json")
	endpointsBlob, err := json.Marshal(EndpointInventory{
		ExecutionNodes: []ExecutionNode{{
			Index:    0,
			ELClient: "reth",
			RPC:      rpcServer.URL,
		}},
	})
	if err != nil {
		t.Fatalf("marshal endpoints: %v", err)
	}
	if err := os.WriteFile(endpointsPath, endpointsBlob, 0o644); err != nil {
		t.Fatalf("write endpoints file: %v", err)
	}

	ctx := newSpamTestContext(t,
		"--accounts-file", accountsPath,
		"--rpc", rpcServer.URL,
		"--txcount", "1",
		"--seed", "1",
		"--accounts", "1",
	)

	cfg, err := NewConfigFromContext(ctx)
	if err != nil {
		t.Fatalf("new config from context: %v", err)
	}
	if cfg.resolvedEndpoint.RPCURL != rpcServer.URL {
		t.Fatalf("unexpected resolved rpc url: %q", cfg.resolvedEndpoint.RPCURL)
	}
	if cfg.resolvedEndpoint.RPCLabel != "explicit-rpc" {
		t.Fatalf("unexpected resolved rpc label: %q", cfg.resolvedEndpoint.RPCLabel)
	}
	if cfg.resolvedEndpoint.Source != "" {
		t.Fatalf("unexpected resolved source: %q", cfg.resolvedEndpoint.Source)
	}
}
