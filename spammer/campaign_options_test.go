package spammer

import (
	stdflag "flag"
	"strings"
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
	if opts.ExecutionMode != "legacy" || opts.MaxInFlight != 8 || opts.ConfirmSLA != 2*time.Second || opts.ConfirmDrain != 5*time.Second {
		t.Fatalf("unexpected d1 execution defaults: %#v", opts)
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
	if basic.ExecutionMode != "legacy" || basic.MaxInFlight != 8 || basic.ConfirmSLA != 2*time.Second || basic.ConfirmDrain != 5*time.Second {
		t.Fatalf("unexpected basic v2 defaults: %#v", basic)
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

func TestCampaignOptionsFromContextParsesD1ExecutionControls(t *testing.T) {
	ctx := newCampaignTestContext(t,
		"--execution-mode", "v2-single-lane",
		"--max-inflight", "17",
		"--confirm-sla", "9s",
		"--confirm-drain-timeout", "13s",
		"--receipt-timeout", "11s",
	)

	opts := campaignOptionsFromContext(ctx, "basic")
	if opts.ExecutionMode != "v2-single-lane" {
		t.Fatalf("unexpected execution mode: %#v", opts)
	}
	if opts.MaxInFlight != 17 || opts.ConfirmSLA != 9*time.Second || opts.ConfirmDrain != 13*time.Second || opts.ReceiptTimeout != 11*time.Second {
		t.Fatalf("unexpected execution controls: %#v", opts)
	}
}

func TestNormalizeCampaignOptionsV2ReceiptTimeoutDefaultsToConfirmDrain(t *testing.T) {
	opts := normalizeCampaignOptions(CampaignOptions{
		ExecutionMode: CampaignExecutionModeV2SingleLane,
		ConfirmDrain:  13 * time.Second,
	})
	if opts.ReceiptTimeout != 13*time.Second {
		t.Fatalf("receipt timeout should default to confirm drain in v2: %#v", opts)
	}
}

func TestValidateCampaignExecutionModeRestrictsV2ToBasic(t *testing.T) {
	if err := validateCampaignExecutionMode(CampaignOptions{TxFamily: "basic", ExecutionMode: CampaignExecutionModeV2SingleLane}); err != nil {
		t.Fatalf("basic should allow v2-single-lane: %v", err)
	}
	for _, family := range []string{"blob", "pectra"} {
		err := validateCampaignExecutionMode(CampaignOptions{TxFamily: family, ExecutionMode: CampaignExecutionModeV2SingleLane})
		if err == nil || !strings.Contains(err.Error(), "only supported for campaign basic") {
			t.Fatalf("%s should reject v2-single-lane, got err=%v", family, err)
		}
	}
}
