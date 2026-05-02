package spammer

import (
	"context"
	"crypto/ecdsa"
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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
	"github.com/urfave/cli/v2"
)

type CampaignOptions struct {
	CampaignID     string
	Cases          int
	TxFamily       string
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
	return RunCampaignFamilyFromContext(c, "basic")
}

func RunBlobCampaignFromContext(c *cli.Context) error {
	return RunCampaignFamilyFromContext(c, "blob")
}

func RunPectraCampaignFromContext(c *cli.Context) error {
	return RunCampaignFamilyFromContext(c, "pectra")
}

func RunCampaignFamilyFromContext(c *cli.Context, family string) error {
	config, err := NewConfigFromContext(c)
	if err != nil {
		return err
	}
	return RunCampaign(config, CampaignOptions{
		CampaignID:     c.String(flags.CampaignIDFlag.Name),
		Cases:          c.Int(flags.CasesFlag.Name),
		TxFamily:       family,
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
	if opts.TxFamily == "" {
		opts.TxFamily = "basic"
	}
	return RunCampaign(config, opts)
}

func RunCampaign(config *Config, opts CampaignOptions) error {
	if opts.CampaignID == "" {
		opts.CampaignID = runner.NewCampaignID()
	}
	if opts.Cases <= 0 {
		opts.Cases = 1
	}
	if opts.TxFamily == "" {
		opts.TxFamily = "basic"
	}
	if opts.ForkLabel == "" {
		if opts.TxFamily == "pectra" {
			opts.ForkLabel = "prague"
		} else {
			opts.ForkLabel = "cancun"
		}
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
	builder := &campaignBuilder{config: config, client: client, chainID: chainID, options: opts}
	submitter := &campaignSubmitter{client: client, rpcLabel: opts.RPCLabel, receiptTimeout: opts.ReceiptTimeout}
	sink := &campaignSink{
		artifactRoot: opts.ArtifactRoot,
		replayDir:    opts.ReplayDir,
		store:        corpus.NewStore(opts.RetainDir, opts.RetainPerSig),
	}
	stats, err := runner.RunCampaign(context.Background(), runner.Config{
		CampaignID: opts.CampaignID,
		Cases:      opts.Cases,
		TxFamily:   opts.TxFamily,
		ForkLabel:  opts.ForkLabel,
	}, builder, submitter, sink)
	if err != nil {
		return err
	}
	return runner.WriteReport(opts.ReportJSON, stats)
}

type campaignBuilder struct {
	config  *Config
	client  *ethclient.Client
	chainID *big.Int
	options CampaignOptions
}

func (b *campaignBuilder) Build(ctx context.Context, sequence int) (*runner.Case, error) {
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
	tx, auths, signer, family, err := buildCampaignTx(ctx, b.config, b.client, b.chainID, b.options.TxFamily, key, sender, nonce, fill, sequence)
	if err != nil {
		return nil, err
	}
	signedTx, err := types.SignTx(tx, signer, key)
	if err != nil {
		return nil, err
	}
	rawTx, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	feeFieldMap := feeFields(signedTx)
	record := runner.TestcaseRecord{
		CaseID:            runner.NewCaseID(sequence),
		CampaignID:        b.options.CampaignID,
		RunStartedAt:      time.Now().UTC(),
		Sequence:          sequence,
		TxFamily:          family,
		ForkLabel:         b.options.ForkLabel,
		SourceKind:        sourceKind,
		Seed:              b.config.seed,
		CorpusInputRef:    corpusRef,
		Sender:            sender.Hex(),
		Nonce:             signedTx.Nonce(),
		GasLimit:          signedTx.Gas(),
		AccessListEnabled: b.config.accessList,
		ValueWei:          signedTx.Value().String(),
		FeeFields:         feeFieldMap,
		Mutation:          mutation,
		UnsignedSummary:   summarizeTx(signedTx),
		SignedTxHex:       hexutil.Encode(rawTx),
		SignedTxHash:      signedTx.Hash().Hex(),
	}
	applyFamilyMetadata(family, feeFieldMap, signedTx, &record, auths)
	return &runner.Case{
		Record: record,
		Tx:     signedTx,
		RawTx:  rawTx,
	}, nil
}

type campaignSigner interface {
	Sender(tx *types.Transaction) (common.Address, error)
}

func buildCampaignTx(ctx context.Context, config *Config, client *ethclient.Client, chainID *big.Int, family string, key *ecdsa.PrivateKey, sender common.Address, nonce uint64, fill *filler.Filler, sequence int) (*types.Transaction, []types.SetCodeAuthorization, campaignSigner, string, error) {
	switch family {
	case "", "basic":
		tx, err := txfuzz.RandomValidTx(config.backend, fill, sender, nonce, nil, nil, config.accessList)
		return tx, nil, types.NewCancunSigner(chainID), "basic", err
	case "blob":
		tx, err := txfuzz.RandomBlobTx(config.backend, fill, sender, nonce, nil, nil, config.accessList)
		return tx, nil, types.NewCancunSigner(chainID), "blob", err
	case "pectra":
		auths, err := buildSetCodeAuthorizations(ctx, config, client, chainID, sender, sequence)
		if err != nil {
			return nil, nil, nil, "pectra", err
		}
		tx, err := txfuzz.RandomAuthTx(config.backend, fill, sender, nonce, nil, nil, config.accessList, auths)
		return tx, auths, types.NewPragueSigner(chainID), "pectra", err
	default:
		return nil, nil, nil, family, fmt.Errorf("unsupported campaign tx family: %s", family)
	}
}

func buildSetCodeAuthorizations(ctx context.Context, config *Config, client *ethclient.Client, chainID *big.Int, sender common.Address, sequence int) ([]types.SetCodeAuthorization, error) {
	if len(config.keys) == 0 {
		return nil, fmt.Errorf("missing authorizer keys")
	}
	authorizer := config.keys[(sequence-1)%len(config.keys)]
	authorizerAddr := crypto.PubkeyToAddress(authorizer.PublicKey)
	nonceAuth, err := client.PendingNonceAt(ctx, authorizerAddr)
	if err != nil {
		return nil, err
	}
	auth := types.SetCodeAuthorization{ChainID: *uint256.MustFromBig(chainID), Address: sender, Nonce: nonceAuth}
	auth, err = types.SignSetCode(authorizer, auth)
	if err != nil {
		return nil, err
	}
	return []types.SetCodeAuthorization{auth}, nil
}

func applyFamilyMetadata(family string, feeFieldMap map[string]string, tx *types.Transaction, record *runner.TestcaseRecord, auths []types.SetCodeAuthorization) {
	if record == nil || tx == nil {
		return
	}
	switch family {
	case "blob":
		record.BlobCount = len(tx.BlobHashes())
		if blobFeeCap := tx.BlobGasFeeCap(); blobFeeCap != nil {
			feeFieldMap["blob_fee_cap"] = blobFeeCap.String()
		}
	case "pectra":
		if len(auths) == 0 {
			auths = tx.SetCodeAuthorizations()
		}
		record.AuthorizationCount = len(auths)
	}
}

type campaignSubmitter struct {
	client         *ethclient.Client
	rpcLabel       string
	receiptTimeout time.Duration
}

func (s *campaignSubmitter) Submit(ctx context.Context, c *runner.Case) (feedback.Record, error) {
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
