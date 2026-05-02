package spammer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	txfuzz "github.com/MariusVanDerWijden/tx-fuzz"
	"github.com/MariusVanDerWijden/tx-fuzz/corpus"
	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/MariusVanDerWijden/tx-fuzz/flags"
	"github.com/MariusVanDerWijden/tx-fuzz/replay"
	"github.com/MariusVanDerWijden/tx-fuzz/runner"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

type CampaignOptions struct {
	CampaignID     string
	Cases          int
	ForkLabel      string
	ArtifactRoot   string
	RetainDir      string
	ReplayDir      string
	ReportJSON     string
	RPCLabel       string
	RetainPerSig   int
	ReceiptTimeout time.Duration
}

func RunBasicCampaignFromContext(c *cli.Context) error {
	config, err := NewConfigFromContext(c)
	if err != nil {
		return err
	}
	return RunBasicCampaign(config, CampaignOptions{
		CampaignID:     c.String(flags.CampaignIDFlag.Name),
		Cases:          c.Int(flags.CasesFlag.Name),
		ForkLabel:      c.String(flags.ForkLabelFlag.Name),
		ArtifactRoot:   c.String(flags.ArtifactRootFlag.Name),
		RetainDir:      c.String(flags.RetainDirFlag.Name),
		ReplayDir:      c.String(flags.ReplayDirFlag.Name),
		ReportJSON:     c.String(flags.ReportJSONFlag.Name),
		RPCLabel:       c.String(flags.RpcLabelFlag.Name),
		RetainPerSig:   c.Int(flags.RetainPerSigFlag.Name),
		ReceiptTimeout: 2 * time.Second,
	})
}

func RunBasicCampaign(config *Config, opts CampaignOptions) error {
	if opts.CampaignID == "" {
		opts.CampaignID = runner.NewCampaignID()
	}
	if opts.Cases <= 0 {
		opts.Cases = 1
	}
	if opts.ForkLabel == "" {
		opts.ForkLabel = "cancun"
	}
	if opts.RPCLabel == "" {
		opts.RPCLabel = "default-rpc"
	}
	if opts.ReceiptTimeout <= 0 {
		opts.ReceiptTimeout = 2 * time.Second
	}
	if opts.RetainPerSig <= 0 {
		opts.RetainPerSig = 1
	}

	client := ethclient.NewClient(config.backend)
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return err
	}
	builder := &basicCampaignBuilder{config: config, client: client, chainID: chainID, options: opts}
	submitter := &basicCampaignSubmitter{client: client, rpcLabel: opts.RPCLabel, receiptTimeout: opts.ReceiptTimeout}
	sink := &campaignSink{
		artifactRoot: opts.ArtifactRoot,
		replayDir:    opts.ReplayDir,
		store:        corpus.NewStore(opts.RetainDir, opts.RetainPerSig),
	}
	stats, err := runner.RunCampaign(context.Background(), runner.Config{
		CampaignID: opts.CampaignID,
		Cases:      opts.Cases,
		TxFamily:   "basic",
		ForkLabel:  opts.ForkLabel,
	}, builder, submitter, sink)
	if err != nil {
		return err
	}
	return runner.WriteReport(opts.ReportJSON, stats)
}

type basicCampaignBuilder struct {
	config  *Config
	client  *ethclient.Client
	chainID *big.Int
	options CampaignOptions
}

func (b *basicCampaignBuilder) Build(ctx context.Context, sequence int) (*runner.Case, error) {
	key := b.config.faucet
	if len(b.config.keys) > 0 {
		key = b.config.keys[(sequence-1)%len(b.config.keys)]
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := b.client.PendingNonceAt(ctx, sender)
	if err != nil {
		return nil, err
	}
	fill, mutation, corpusRef, sourceKind := buildCampaignFiller(b.config)
	tx, err := txfuzz.RandomValidTx(b.config.backend, fill, sender, nonce, nil, nil, b.config.accessList)
	if err != nil {
		return nil, err
	}
	signedTx, err := types.SignTx(tx, types.NewCancunSigner(b.chainID), key)
	if err != nil {
		return nil, err
	}
	rawTx, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &runner.Case{
		Record: runner.TestcaseRecord{
			CaseID:            runner.NewCaseID(sequence),
			CampaignID:        b.options.CampaignID,
			RunStartedAt:      time.Now().UTC(),
			Sequence:          sequence,
			TxFamily:          "basic",
			ForkLabel:         b.options.ForkLabel,
			SourceKind:        sourceKind,
			Seed:              b.config.seed,
			CorpusInputRef:    corpusRef,
			Sender:            sender.Hex(),
			Nonce:             signedTx.Nonce(),
			GasLimit:          signedTx.Gas(),
			AccessListEnabled: b.config.accessList,
			ValueWei:          signedTx.Value().String(),
			FeeFields:         feeFields(signedTx),
			Mutation:          mutation,
			UnsignedSummary:   summarizeTx(signedTx),
			SignedTxHex:       hexutil.Encode(rawTx),
			SignedTxHash:      signedTx.Hash().Hex(),
		},
		Tx:    signedTx,
		RawTx: rawTx,
	}, nil
}

type basicCampaignSubmitter struct {
	client         *ethclient.Client
	rpcLabel       string
	receiptTimeout time.Duration
}

func (s *basicCampaignSubmitter) Submit(ctx context.Context, c *runner.Case) (feedback.Record, error) {
	start := time.Now().UTC()
	record := feedback.Record{
		CaseID:          c.Record.CaseID,
		RPCLabel:        s.rpcLabel,
		SubmitStartedAt: start,
	}
	if c.Tx == nil {
		record.SendStatus = "internal_error"
		record.RPCErrorClass = "missing_tx"
		record.RPCErrorMessage = "runner case missing signed transaction"
		return record, nil
	}
	if err := s.client.SendTransaction(ctx, c.Tx); err != nil {
		record.SubmitLatencyMS = time.Since(start).Milliseconds()
		record.SendStatus = "rpc_error"
		record.RPCErrorClass = classifyRPCError(err)
		record.RPCErrorMessage = err.Error()
		return record, nil
	}
	record.SubmitLatencyMS = time.Since(start).Milliseconds()
	record.SendStatus = "sent"
	receiptCtx, cancel := context.WithTimeout(ctx, s.receiptTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(receiptCtx, s.client, c.Tx)
	if err != nil {
		if receiptCtx.Err() != nil {
			record.AnomalyFlags = append(record.AnomalyFlags, "receipt_timeout")
			return record, nil
		}
		record.AnomalyFlags = append(record.AnomalyFlags, "receipt_lookup_failed")
		record.ProcessSignals.StderrHints = append(record.ProcessSignals.StderrHints, err.Error())
		return record, nil
	}
	if receipt != nil {
		status := receipt.Status
		gasUsed := receipt.GasUsed
		latency := time.Since(start).Milliseconds()
		record.ReceiptObserved = true
		record.ReceiptStatus = &status
		record.ReceiptGasUsed = &gasUsed
		record.InclusionLatencyMS = &latency
		if status == types.ReceiptStatusSuccessful {
			record.ExecutionBucket = "success"
		} else {
			record.ExecutionBucket = "reverted"
		}
	}
	return record, nil
}

type campaignSink struct {
	artifactRoot string
	replayDir    string
	store        *corpus.Store
}

func (s *campaignSink) Accept(_ context.Context, outcome runner.Outcome) (runner.SinkResult, error) {
	if _, _, err := runner.WriteCaseArtifact(s.artifactRoot, outcome.Case, outcome.Feedback); err != nil {
		return runner.SinkResult{}, err
	}
	retained := runner.RetainedCase{
		Case:       outcome.Case,
		Feedback:   outcome.Feedback,
		Signature:  outcome.Signature,
		Score:      outcome.Score,
		Reasons:    outcome.Reasons,
		RetainedAt: time.Now().UTC(),
	}
	decision := s.store.Decide(retained)
	if decision.Retain {
		bundlePath, err := replay.ExportBundle(s.replayDir, retained, replay.EnvironmentSpec{ClientLabel: outcome.Feedback.RPCLabel, ForkLabel: outcome.Case.ForkLabel})
		if err != nil {
			return runner.SinkResult{}, err
		}
		retained.ReplayRef = bundlePath
		if err := s.store.Save(retained); err != nil {
			return runner.SinkResult{}, err
		}
	}
	return runner.SinkResult{Retained: decision.Retain, Duplicate: decision.Duplicate}, nil
}

func ReplayBundle(bundlePath, rpcURL string) (*replay.Bundle, error) {
	bundle, err := replay.LoadBundle(bundlePath)
	if err != nil {
		return nil, err
	}
	if rpcURL == "" {
		rpcURL = bundle.Environment.RPCURL
	}
	if rpcURL == "" {
		rpcURL = bundle.Environment.RPCURL
	}
	if rpcURL == "" {
		return nil, fmt.Errorf("replay requires an explicit rpc URL or bundle environment rpc_url")
	}
	rawPath := filepath.Join(filepath.Dir(bundlePath), "tx.rlp")
	rawBytes, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, err
	}
	if len(rawBytes) == 0 {
		return &bundle, nil
	}
	client, err := rpc.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	if err := client.CallContext(context.Background(), nil, "eth_sendRawTransaction", hexutil.Encode(rawBytes)); err != nil {
		return &bundle, err
	}
	return &bundle, nil
}

func buildCampaignFiller(config *Config) (*filler.Filler, runner.MutationRecord, string, string) {
	base := make([]byte, 10_000)
	config.mut.FillBytes(&base)
	sourceKind := "random"
	corpusRef := ""
	if len(config.corpus) != 0 {
		idx := mrand.Int31n(int32(len(config.corpus)))
		base = append([]byte(nil), config.corpus[idx]...)
		sourceKind = "corpus"
		corpusRef = fmt.Sprintf("corpus-%d", idx)
	}
	mutated := append([]byte(nil), base...)
	config.mut.MutateBytes(&mutated)
	sum := sha256.Sum256(base)
	return filler.NewFiller(mutated), runner.MutationRecord{
		BaseInputHash: hexutil.Encode(sum[:]),
		MutatorNames:  []string{"mutator.MutateBytes"},
		MutationCount: 1,
	}, corpusRef, sourceKind
}

func summarizeTx(tx *types.Transaction) runner.TxSummary {
	to := ""
	if tx.To() != nil {
		to = tx.To().Hex()
	}
	return runner.TxSummary{
		To:               to,
		ContractCreation: tx.To() == nil,
		DataLen:          len(tx.Data()),
		AccessListSize:   len(tx.AccessList()),
		TxType:           tx.Type(),
	}
}

func feeFields(tx *types.Transaction) map[string]string {
	fields := map[string]string{}
	if gasPrice := tx.GasPrice(); gasPrice != nil {
		fields["gas_price"] = gasPrice.String()
	}
	if tip := tx.GasTipCap(); tip != nil {
		fields["gas_tip_cap"] = tip.String()
	}
	if feeCap := tx.GasFeeCap(); feeCap != nil {
		fields["gas_fee_cap"] = feeCap.String()
	}
	return fields
}

func classifyRPCError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "nonce too low"):
		return "nonce_too_low"
	case strings.Contains(msg, "replacement transaction underpriced"):
		return "replacement_underpriced"
	case strings.Contains(msg, "insufficient funds"):
		return "insufficient_funds"
	default:
		return "rpc_error"
	}
}
