package spammer

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	return RunCampaign(config, campaignOptionsFromContext(c, family))
}

func campaignOptionsFromContext(c *cli.Context, family string) CampaignOptions {
	return CampaignOptions{
		CampaignID:     stringFlagValue(c, flags.CampaignIDFlag),
		Cases:          intFlagValue(c, flags.CasesFlag),
		TxFamily:       family,
		ForkLabel:      stringFlagValue(c, flags.ForkLabelFlag),
		ArtifactRoot:   stringFlagValue(c, flags.ArtifactRootFlag),
		RetainDir:      stringFlagValue(c, flags.RetainDirFlag),
		ReplayDir:      stringFlagValue(c, flags.ReplayDirFlag),
		ReportJSON:     stringFlagValue(c, flags.ReportJSONFlag),
		RPCLabel:       stringFlagValue(c, flags.RpcLabelFlag),
		RetainPerSig:   intFlagValue(c, flags.RetainPerSigFlag),
		ReceiptTimeout: 2 * time.Second,
	}
}

func stringFlagValue(c *cli.Context, flag *cli.StringFlag) string {
	for _, name := range flag.Names() {
		if c.IsSet(name) {
			return c.String(name)
		}
	}
	return c.String(flag.Name)
}

func intFlagValue(c *cli.Context, flag *cli.IntFlag) int {
	for _, name := range flag.Names() {
		if c.IsSet(name) {
			return c.Int(name)
		}
	}
	return c.Int(flag.Name)
}

func normalizeCampaignOptions(opts CampaignOptions) CampaignOptions {
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
	return opts
}

func RunBasicCampaign(config *Config, opts CampaignOptions) error {
	if opts.TxFamily == "" {
		opts.TxFamily = "basic"
	}
	return RunCampaign(config, opts)
}

func RunCampaign(config *Config, opts CampaignOptions) error {
	opts = normalizeCampaignOptions(opts)

	client := ethclient.NewClient(config.backend)
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return err
	}
	builder := &campaignBuilder{
		config:  config,
		client:  client,
		chainID: chainID,
		options: opts,
		nonce:   runner.NewOrchestrator(client),
	}
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
	nonce   *runner.Orchestrator
}

func (b *campaignBuilder) Build(ctx context.Context, sequence int) (*runner.Case, error) {
	key := b.config.faucet
	if len(b.config.keys) > 0 {
		key = b.config.keys[(sequence-1)%len(b.config.keys)]
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	lane := b.nonce.NewLane()
	nonce, err := lane.Lease(ctx, sender)
	if err != nil {
		return nil, err
	}
	fill, mutation, corpusRef, sourceKind := buildCampaignFiller(b.config)
	tx, auths, signer, family, err := buildCampaignTx(ctx, b.config, b.client, b.chainID, b.options.TxFamily, key, sender, nonce, fill, sequence, lane)
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

func buildCampaignTx(ctx context.Context, config *Config, client *ethclient.Client, chainID *big.Int, family string, _ *ecdsa.PrivateKey, sender common.Address, nonce uint64, fill *filler.Filler, sequence int, lane *runner.Lane) (*types.Transaction, []types.SetCodeAuthorization, types.Signer, string, error) {
	switch family {
	case "", "basic":
		tx, err := txfuzz.RandomValidTx(config.backend, fill, sender, nonce, nil, nil, config.accessList)
		return tx, nil, types.NewCancunSigner(chainID), "basic", err
	case "blob":
		tx, err := txfuzz.RandomBlobTx(config.backend, fill, sender, nonce, nil, nil, config.accessList)
		return tx, nil, types.NewCancunSigner(chainID), "blob", err
	case "pectra":
		auths, err := buildSetCodeAuthorizations(ctx, config, chainID, sender, sequence, lane)
		if err != nil {
			return nil, nil, nil, "pectra", err
		}
		tx, err := txfuzz.RandomAuthTx(config.backend, fill, sender, nonce, nil, nil, config.accessList, auths)
		return tx, auths, types.NewPragueSigner(chainID), "pectra", err
	default:
		return nil, nil, nil, family, fmt.Errorf("unsupported campaign tx family: %s", family)
	}
}

func buildSetCodeAuthorizations(ctx context.Context, config *Config, chainID *big.Int, sender common.Address, sequence int, lane *runner.Lane) ([]types.SetCodeAuthorization, error) {
	if len(config.keys) == 0 {
		return nil, fmt.Errorf("missing authorizer keys")
	}
	authorizer := config.keys[(sequence-1)%len(config.keys)]
	authorizerAddr := crypto.PubkeyToAddress(authorizer.PublicKey)
	nonceAuth, err := lane.Lease(ctx, authorizerAddr)
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
	artifactRoot   string
	sendTx         func(context.Context, *types.Transaction) error
	waitMined      func(context.Context, *ethclient.Client, *types.Transaction) (*types.Receipt, error)
	lane           chan struct{}
}

type campaignEvent struct {
	CaseID          string   `json:"case_id"`
	CampaignID      string   `json:"campaign_id,omitempty"`
	Stage           string   `json:"stage"`
	SendStatus      string   `json:"send_status"`
	ReceiptObserved bool     `json:"receipt_observed"`
	AnomalyFlags    []string `json:"anomaly_flags,omitempty"`
}

func (s *campaignSubmitter) Submit(ctx context.Context, c *runner.Case) (feedback.Record, error) {
	pending, err := s.SubmitAsync(ctx, c)
	if err != nil {
		return feedback.Record{}, err
	}
	return pending.Await(ctx)
}

type pendingCampaignSubmission struct {
	recordCh <-chan feedback.Record
	errCh    <-chan error
	once     sync.Once
	record   feedback.Record
	err      error
}

func (p *pendingCampaignSubmission) Await(ctx context.Context) (feedback.Record, error) {
	p.once.Do(func() {
		select {
		case <-ctx.Done():
			p.err = ctx.Err()
		case err := <-p.errCh:
			p.err = err
		case record := <-p.recordCh:
			p.record = record
		}
	})
	return p.record, p.err
}

func (s *campaignSubmitter) SubmitAsync(ctx context.Context, c *runner.Case) (runner.PendingSubmission, error) {
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
		return completedPendingSubmission(record), nil
	}
	lane := s.ensureLane()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case lane <- struct{}{}:
	}
	sendTx := s.sendTransaction
	if err := sendTx(ctx, c.Tx); err != nil {
		s.releaseLane()
		record.SubmitLatencyMS = time.Since(start).Milliseconds()
		record.SendStatus = "rpc_error"
		record.RPCErrorClass = classifyRPCError(err)
		record.RPCErrorMessage = err.Error()
		return completedPendingSubmission(record), nil
	}
	record.SubmitLatencyMS = time.Since(start).Milliseconds()
	record.SendStatus = "sent"
	if err := s.appendEvent(c.Record, record, "submitted"); err != nil {
		s.releaseLane()
		return nil, err
	}
	recordCh := make(chan feedback.Record, 1)
	errCh := make(chan error, 1)
	go func(base feedback.Record) {
		defer s.releaseLane()
		receiptCtx, cancel := context.WithTimeout(context.Background(), s.receiptTimeout)
		defer cancel()
		receipt, err := s.waitForMined(receiptCtx, c.Tx)
		if err != nil {
			if receiptCtx.Err() != nil {
				base.AnomalyFlags = append(base.AnomalyFlags, "receipt_timeout")
				_ = s.appendEvent(c.Record, base, "finalized")
				recordCh <- base
				return
			}
			base.AnomalyFlags = append(base.AnomalyFlags, "receipt_lookup_failed")
			base.ProcessSignals.StderrHints = append(base.ProcessSignals.StderrHints, err.Error())
			_ = s.appendEvent(c.Record, base, "finalized")
			recordCh <- base
			return
		}
		if receipt != nil {
			status := receipt.Status
			gasUsed := receipt.GasUsed
			latency := time.Since(start).Milliseconds()
			base.ReceiptObserved = true
			base.ReceiptStatus = &status
			base.ReceiptGasUsed = &gasUsed
			base.InclusionLatencyMS = &latency
			if status == types.ReceiptStatusSuccessful {
				base.ExecutionBucket = "success"
			} else {
				base.ExecutionBucket = "reverted"
			}
		}
		_ = s.appendEvent(c.Record, base, "finalized")
		recordCh <- base
	}(record)
	return &pendingCampaignSubmission{recordCh: recordCh, errCh: errCh}, nil
}

func completedPendingSubmission(record feedback.Record) runner.PendingSubmission {
	recordCh := make(chan feedback.Record, 1)
	errCh := make(chan error, 1)
	recordCh <- record
	return &pendingCampaignSubmission{recordCh: recordCh, errCh: errCh}
}

func (s *campaignSubmitter) ensureLane() chan struct{} {
	if s.lane == nil {
		s.lane = make(chan struct{}, 1)
	}
	return s.lane
}

func (s *campaignSubmitter) releaseLane() {
	if s.lane == nil {
		return
	}
	select {
	case <-s.lane:
	default:
	}
}

func (s *campaignSubmitter) sendTransaction(ctx context.Context, tx *types.Transaction) error {
	if s.sendTx != nil {
		return s.sendTx(ctx, tx)
	}
	return s.client.SendTransaction(ctx, tx)
}

func (s *campaignSubmitter) waitForMined(ctx context.Context, tx *types.Transaction) (*types.Receipt, error) {
	if s.waitMined != nil {
		return s.waitMined(ctx, s.client, tx)
	}
	return bind.WaitMined(ctx, s.client, tx)
}

func (s *campaignSubmitter) appendEvent(record runner.TestcaseRecord, fb feedback.Record, stage string) error {
	if strings.TrimSpace(s.artifactRoot) == "" {
		return nil
	}
	if err := os.MkdirAll(s.artifactRoot, 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(s.artifactRoot, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	event := campaignEvent{
		CaseID:          record.CaseID,
		CampaignID:      record.CampaignID,
		Stage:           stage,
		SendStatus:      fb.SendStatus,
		ReceiptObserved: fb.ReceiptObserved,
		AnomalyFlags:    fb.AnomalyFlags,
	}
	blob, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := writer.Write(blob); err != nil {
		return err
	}
	if err := writer.WriteByte('\n'); err != nil {
		return err
	}
	return writer.Flush()
}

type campaignSink struct {
	artifactRoot string
	replayDir    string
	store        *corpus.Store
}

func (s *campaignSink) Accept(_ context.Context, outcome runner.Outcome) (runner.SinkResult, error) {
	if err := runner.WriteCaseMetadataArtifact(s.artifactRoot, outcome.Case); err != nil {
		return runner.SinkResult{}, err
	}
	if runner.IsCanonicalFeedbackTerminal(outcome.Feedback) {
		if err := runner.WriteFeedbackArtifact(s.artifactRoot, outcome.Feedback); err != nil {
			return runner.SinkResult{}, err
		}
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
