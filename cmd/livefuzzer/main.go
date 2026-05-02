package main

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/flags"
	"github.com/MariusVanDerWijden/tx-fuzz/spammer"
	"github.com/ethereum/go-ethereum/params"
	"github.com/urfave/cli/v2"
)

var airdropCommand = &cli.Command{
	Name:   "airdrop",
	Usage:  "Airdrops to a list of accounts",
	Action: runAirdrop,
	Flags: []cli.Flag{
		flags.SkFlag,
		flags.RpcFlag,
		flags.EndpointsFlag,
		flags.ELClientFlag,
		flags.CountFlag,
		flags.TxCountFlag,
		flags.GasLimitFlag,
	},
}

var spamCommand = &cli.Command{
	Name:   "spam",
	Usage:  "Send spam transactions",
	Action: runBasicSpam,
	Flags:  flags.SpamFlags,
}

var blobSpamCommand = &cli.Command{
	Name:   "blobs",
	Usage:  "Send blob spam transactions",
	Action: runBlobSpam,
	Flags:  flags.SpamFlags,
}

var pectraSpamCommand = &cli.Command{
	Name:   "pectra",
	Usage:  "Send 7702 spam transactions",
	Action: run7702Spam,
	Flags:  flags.SpamFlags,
}

var campaignCommand = &cli.Command{
	Name:  "campaign",
	Usage: "Run a bounded artifact-producing fuzz campaign",
	Subcommands: []*cli.Command{
		{
			Name:   "basic",
			Usage:  "Run a bounded basic transaction campaign",
			Action: runBasicCampaign,
			Flags:  flags.CampaignFlags,
		},
	},
}

var replayCommand = &cli.Command{
	Name:   "replay",
	Usage:  "Replay a retained bundle",
	Action: runReplay,
	Flags: []cli.Flag{
		flags.BundleFlag,
		flags.RpcFlag,
		flags.EndpointsFlag,
		flags.ELClientFlag,
		flags.RpcLabelFlag,
	},
}

var createCommand = &cli.Command{
	Name:   "create",
	Usage:  "Create ephemeral accounts",
	Action: runCreate,
	Flags: []cli.Flag{
		flags.CountFlag,
		flags.RpcFlag,
	},
}

var unstuckCommand = &cli.Command{
	Name:   "unstuck",
	Usage:  "Tries to unstuck an account",
	Action: runUnstuck,
	Flags:  flags.SpamFlags,
}

func initApp() *cli.App {
	app := cli.NewApp()
	app.Name = "tx-fuzz"
	app.Usage = "Fuzzer for sending spam transactions"
	app.Commands = []*cli.Command{
		airdropCommand,
		spamCommand,
		blobSpamCommand,
		pectraSpamCommand,
		campaignCommand,
		replayCommand,
		createCommand,
		unstuckCommand,
	}
	return app
}

var app = initApp()

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAirdrop(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	txPerAccount := config.N
	airdropValue := new(big.Int).Mul(big.NewInt(int64(txPerAccount*100000)), big.NewInt(params.GWei))
	spammer.Airdrop(config, airdropValue)
	return nil
}

func spam(config *spammer.Config, spamFn spammer.Spam, airdropValue *big.Int) error {
	fmt.Println("Unstucking")
	spammer.Unstuck(config)
	for {
		fmt.Println("Airdropping")
		if err := spammer.Airdrop(config, airdropValue); err != nil {
			return err
		}
		fmt.Println("Spamming")
		spammer.SpamTransactions(config, spamFn)
		time.Sleep(time.Duration(config.SlotTime) * time.Second)
	}
}

func runBasicSpam(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+config.N)*1000000)), big.NewInt(params.GWei))
	return spam(config, spammer.SendBasicTransactions, airdropValue)
}

func runBlobSpam(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+config.N)*1000000)), big.NewInt(params.GWei))
	airdropValue = airdropValue.Mul(airdropValue, big.NewInt(100))
	return spam(config, spammer.SendBlobTransactions, airdropValue)
}

func run7702Spam(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+config.N)*1000000)), big.NewInt(params.GWei))
	airdropValue = airdropValue.Mul(airdropValue, big.NewInt(100))
	return spam(config, spammer.Send7702Transactions, airdropValue)
}

func runBasicCampaign(c *cli.Context) error {
	return spammer.RunBasicCampaignFromContext(c)
}

func runReplay(c *cli.Context) error {
	resolved, err := spammer.ResolveEndpointSelection(
		c.String(flags.RpcFlag.Name),
		c.String(flags.RpcLabelFlag.Name),
		c.String(flags.EndpointsFlag.Name),
		c.String(flags.ELClientFlag.Name),
	)
	if err != nil {
		return err
	}
	bundle, err := spammer.ReplayBundle(c.String(flags.BundleFlag.Name), resolved.RPCURL)
	if err != nil {
		fmt.Fprintf(c.App.Writer, "bundle=%s case=%s signature=%s rpc=%s label=%s replay_error=%v\n", c.String(flags.BundleFlag.Name), bundle.Case.CaseID, bundle.Signature.StableKey, resolved.RPCURL, resolved.RPCLabel, err)
		return nil
	}
	fmt.Fprintf(c.App.Writer, "bundle=%s case=%s signature=%s rpc=%s label=%s\n", c.String(flags.BundleFlag.Name), bundle.Case.CaseID, bundle.Signature.StableKey, resolved.RPCURL, resolved.RPCLabel)
	return nil
}

func runCreate(c *cli.Context) error {
	spammer.CreateAddresses(100)
	return nil
}

func runUnstuck(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	return spammer.Unstuck(config)
}
