package spammer

import (
	stdflag "flag"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/flags"
	"github.com/urfave/cli/v2"
)

func newCampaignTestContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()
	fs := stdflag.NewFlagSet("campaign-test", stdflag.ContinueOnError)
	for _, f := range flags.CampaignFlags {
		if err := f.Apply(fs); err != nil {
			t.Fatalf("apply flag %v: %v", f.Names(), err)
		}
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse args: %v", err)
	}
	return cli.NewContext(cli.NewApp(), fs, nil)
}

func TestCampaignOptionsFromContextPreservesSharedCampaignSurface(t *testing.T) {
	ctx := newCampaignTestContext(t,
		"--campaign-id", "campaign-123",
		"--cases", "7",
		"--fork-label", "custom-fork",
		"--artifact-root", "/tmp/artifacts",
		"--retain-dir", "/tmp/retain",
		"--replay-dir", "/tmp/replay",
		"--report-json", "/tmp/report.json",
		"--rpc-label", "rpc-a",
		"--retain-per-signature", "3",
	)

	opts := campaignOptionsFromContext(ctx, "blob")
	if opts.CampaignID != "campaign-123" || opts.Cases != 7 || opts.TxFamily != "blob" {
		t.Fatalf("unexpected campaign identity: %#v", opts)
	}
	if opts.ForkLabel != "custom-fork" || opts.ArtifactRoot != "/tmp/artifacts" || opts.RetainDir != "/tmp/retain" {
		t.Fatalf("unexpected artifact wiring: %#v", opts)
	}
	if opts.ReplayDir != "/tmp/replay" || opts.ReportJSON != "/tmp/report.json" || opts.RPCLabel != "rpc-a" {
		t.Fatalf("unexpected report/replay wiring: %#v", opts)
	}
	if opts.RetainPerSig != 3 || opts.ReceiptTimeout != 2*time.Second {
		t.Fatalf("unexpected retention/timeout wiring: %#v", opts)
	}
}

func TestNormalizeCampaignOptionsDefaultsKeepFamilyCompatibility(t *testing.T) {
	basic := normalizeCampaignOptions(CampaignOptions{})
	if basic.CampaignID == "" || basic.Cases != 1 || basic.TxFamily != "basic" || basic.ForkLabel != "cancun" {
		t.Fatalf("unexpected basic defaults: %#v", basic)
	}
	if basic.RPCLabel != "default-rpc" || basic.RetainPerSig != 1 || basic.ReceiptTimeout != 2*time.Second {
		t.Fatalf("unexpected basic service defaults: %#v", basic)
	}

	blob := normalizeCampaignOptions(CampaignOptions{TxFamily: "blob"})
	if blob.TxFamily != "blob" || blob.ForkLabel != "cancun" {
		t.Fatalf("unexpected blob defaults: %#v", blob)
	}

	pectra := normalizeCampaignOptions(CampaignOptions{TxFamily: "pectra"})
	if pectra.TxFamily != "pectra" || pectra.ForkLabel != "prague" {
		t.Fatalf("unexpected pectra defaults: %#v", pectra)
	}

	explicit := normalizeCampaignOptions(CampaignOptions{TxFamily: "pectra", ForkLabel: "override", Cases: 9, RPCLabel: "rpc-z", RetainPerSig: 4, ReceiptTimeout: 5 * time.Second})
	if explicit.ForkLabel != "override" || explicit.Cases != 9 || explicit.RPCLabel != "rpc-z" || explicit.RetainPerSig != 4 || explicit.ReceiptTimeout != 5*time.Second {
		t.Fatalf("explicit values should win: %#v", explicit)
	}
}
