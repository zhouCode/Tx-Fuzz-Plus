package flags

import "github.com/urfave/cli/v2"

var (
	SeedFlag = &cli.Int64Flag{
		Name:  "seed",
		Usage: "Seed for the RNG, (Default = RandomSeed)",
		Value: 0,
	}

	SkFlag = &cli.StringFlag{
		Name:  "sk",
		Usage: "Secret key",
		Value: "0xcdfbe6f7602f67a97602e3e9fc24cde1cdffa88acd47745c0b84c5ff55891e1b",
	}

	CorpusFlag = &cli.StringFlag{
		Name:  "corpus",
		Usage: "Use additional Corpus",
	}

	NoALFlag = &cli.BoolFlag{
		Name:  "no-al",
		Usage: "Disable accesslist creation",
		Value: false,
	}

	CountFlag = &cli.IntFlag{
		Name:  "accounts",
		Usage: "Count of accounts to send transactions from",
		Value: 100,
	}

	RpcFlag = &cli.StringFlag{
		Name:  "rpc",
		Usage: "RPC provider; if omitted, resolve dynamically from endpoints.json",
	}

	EndpointsFlag = &cli.StringFlag{
		Name:  "endpoints",
		Usage: "Path to endpoints.json inventory used to resolve execution node RPC dynamically",
		Value: "~/ethpackage/endpoints.json",
	}

	ELClientFlag = &cli.StringFlag{
		Name:  "el-client",
		Usage: "Execution client label to select from endpoints.json when --rpc is omitted",
	}

	RpcLabelFlag = &cli.StringFlag{
		Name:  "rpc-label",
		Usage: "Label used in feedback artifacts for the RPC target",
		Value: "default-rpc",
	}

	TxCountFlag = &cli.IntFlag{
		Name:  "txcount",
		Usage: "Number of transactions send per account per block, 0 = best estimate",
		Value: 0,
	}

	GasLimitFlag = &cli.IntFlag{
		Name:  "gaslimit",
		Usage: "Gas limit used for transactions",
		Value: 100_000,
	}

	SlotTimeFlag = &cli.IntFlag{
		Name:  "slot-time",
		Usage: "Slot time in seconds",
		Value: 12,
	}

	CampaignIDFlag = &cli.StringFlag{
		Name:  "campaign-id",
		Usage: "Optional explicit campaign identifier",
	}

	CasesFlag = &cli.IntFlag{
		Name:  "cases",
		Usage: "Number of bounded cases to execute in campaign mode",
		Value: 10,
	}

	ForkLabelFlag = &cli.StringFlag{
		Name:  "fork-label",
		Usage: "Fork label stored in campaign artifacts",
		Value: "cancun",
	}

	ArtifactRootFlag = &cli.StringFlag{
		Name:  "artifact-root",
		Usage: "Root directory for per-case metadata and feedback artifacts",
		Value: ".txfuzz/campaign",
	}

	RetainDirFlag = &cli.StringFlag{
		Name:  "retain-dir",
		Usage: "Directory for retained interesting cases",
		Value: ".txfuzz/retain",
	}

	ReplayDirFlag = &cli.StringFlag{
		Name:  "replay-dir",
		Usage: "Directory for replay bundles",
		Value: ".txfuzz/replay",
	}

	ReportJSONFlag = &cli.StringFlag{
		Name:  "report-json",
		Usage: "Path for the campaign JSON report",
		Value: ".txfuzz/report.json",
	}

	RetainPerSigFlag = &cli.IntFlag{
		Name:  "retain-per-signature",
		Usage: "Maximum retained cases per deduplicated signature",
		Value: 1,
	}

	BundleFlag = &cli.StringFlag{
		Name:     "bundle",
		Usage:    "Replay bundle JSON file",
		Required: true,
	}

	SpamFlags = []cli.Flag{
		SkFlag,
		SeedFlag,
		NoALFlag,
		CorpusFlag,
		RpcFlag,
		EndpointsFlag,
		ELClientFlag,
		TxCountFlag,
		CountFlag,
		GasLimitFlag,
		SlotTimeFlag,
	}

	CampaignFlags = []cli.Flag{
		SkFlag,
		SeedFlag,
		NoALFlag,
		CorpusFlag,
		RpcFlag,
		EndpointsFlag,
		ELClientFlag,
		RpcLabelFlag,
		TxCountFlag,
		CountFlag,
		GasLimitFlag,
		CampaignIDFlag,
		CasesFlag,
		ForkLabelFlag,
		ArtifactRootFlag,
		RetainDirFlag,
		ReplayDirFlag,
		ReportJSONFlag,
		RetainPerSigFlag,
	}
)
