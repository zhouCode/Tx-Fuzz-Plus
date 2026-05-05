package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/MariusVanDerWijden/tx-fuzz/interestingness"
)

type MutationRecord struct {
	BaseInputHash string   `json:"base_input_hash"`
	MutatorNames  []string `json:"mutator_names"`
	MutationCount int      `json:"mutation_count"`
	FieldHints    []string `json:"field_hints,omitempty"`
}

type TxSummary struct {
	To               string `json:"to,omitempty"`
	ContractCreation bool   `json:"contract_creation"`
	DataLen          int    `json:"data_len"`
	AccessListSize   int    `json:"access_list_size"`
	TxType           uint8  `json:"tx_type"`
}

type TestcaseRecord struct {
	CaseID               string            `json:"case_id"`
	CampaignID           string            `json:"campaign_id"`
	RunStartedAt         time.Time         `json:"run_started_at"`
	Sequence             int               `json:"sequence"`
	TxFamily             string            `json:"tx_family"`
	ForkLabel            string            `json:"fork_label"`
	SourceKind           string            `json:"source_kind"`
	Seed                 int64             `json:"seed"`
	CorpusInputRef       string            `json:"corpus_input_ref,omitempty"`
	Sender               string            `json:"sender"`
	Nonce                uint64            `json:"nonce"`
	GasLimit             uint64            `json:"gas_limit"`
	AccessListEnabled    bool              `json:"access_list_enabled"`
	ValueWei             string            `json:"value_wei"`
	FeeFields            map[string]string `json:"fee_fields"`
	BlobCount            int               `json:"blob_count,omitempty"`
	AuthorizationCount   int               `json:"authorization_count,omitempty"`
	LaneID               string            `json:"lane_id,omitempty"`
	SenderShard          string            `json:"sender_shard,omitempty"`
	ConsumedNonceDomains []string          `json:"consumed_nonce_domains,omitempty"`
	Mutation             MutationRecord    `json:"mutation"`
	UnsignedSummary      TxSummary         `json:"unsigned_summary"`
	SignedTxHex          string            `json:"signed_tx_hex,omitempty"`
	SignedTxHash         string            `json:"signed_tx_hash,omitempty"`
}

type RetainedCase struct {
	Case       TestcaseRecord                  `json:"case"`
	Feedback   feedback.Record                 `json:"feedback"`
	Signature  interestingness.SignatureRecord `json:"signature"`
	Score      int                             `json:"score"`
	Reasons    []string                        `json:"reasons"`
	ReplayRef  string                          `json:"replay_ref,omitempty"`
	RetainedAt time.Time                       `json:"retained_at"`
}

type Case struct {
	Record TestcaseRecord
	Tx     *types.Transaction
	RawTx  []byte
}

type Config struct {
	CampaignID string
	Cases      int
	TxFamily   string
	ForkLabel  string
}

type Outcome struct {
	Case      TestcaseRecord
	Feedback  feedback.Record
	Signature interestingness.SignatureRecord
	Score     int
	Reasons   []string
}

type Stats struct {
	CampaignID     string
	TxFamily       string
	TotalCases     int
	SentCases      int
	RetainedCases  int
	DuplicateCases int
}

type Builder interface {
	Build(context.Context, int) (*Case, error)
}

type Submitter interface {
	Submit(context.Context, *Case) (feedback.Record, error)
}

type PendingSubmission interface {
	Await(context.Context) (feedback.Record, error)
}

type AsyncSubmitter interface {
	SubmitAsync(context.Context, *Case) (PendingSubmission, error)
}

type SinkResult struct {
	Retained  bool
	Duplicate bool
}

type Sink interface {
	Accept(context.Context, Outcome) (SinkResult, error)
}

func RunCampaign(ctx context.Context, cfg Config, builder Builder, submitter Submitter, sink Sink) (Stats, error) {
	stats := Stats{CampaignID: cfg.CampaignID, TxFamily: cfg.TxFamily}
	asyncSubmitter, async := submitter.(AsyncSubmitter)
	if !async {
		for i := 1; i <= cfg.Cases; i++ {
			stats.TotalCases++
			c, err := builder.Build(ctx, i)
			if err != nil {
				return stats, err
			}
			normalizeCaseFromConfig(c, cfg, i)
			rec, err := submitter.Submit(ctx, c)
			if err != nil {
				return stats, err
			}
			if err := acceptOutcome(ctx, c.Record, rec, sink, &stats); err != nil {
				return stats, err
			}
		}
		return stats, nil
	}

	var (
		current *Case
		err     error
	)
	if cfg.Cases <= 0 {
		return stats, nil
	}
	current, err = builder.Build(ctx, 1)
	if err != nil {
		return stats, err
	}
	normalizeCaseFromConfig(current, cfg, 1)
	for i := 1; i <= cfg.Cases; i++ {
		stats.TotalCases++
		pending, err := asyncSubmitter.SubmitAsync(ctx, current)
		if err != nil {
			return stats, err
		}
		var next *Case
		if i < cfg.Cases {
			next, err = builder.Build(ctx, i+1)
			if err != nil {
				return stats, err
			}
			normalizeCaseFromConfig(next, cfg, i+1)
		}
		rec, err := pending.Await(ctx)
		if err != nil {
			return stats, err
		}
		if err := acceptOutcome(ctx, current.Record, rec, sink, &stats); err != nil {
			return stats, err
		}
		current = next
	}
	return stats, nil
}

func normalizeCaseFromConfig(c *Case, cfg Config, sequence int) {
	if c.Record.CampaignID == "" {
		c.Record.CampaignID = cfg.CampaignID
	}
	if c.Record.CaseID == "" {
		c.Record.CaseID = NewCaseID(sequence)
	}
	if c.Record.TxFamily == "" {
		c.Record.TxFamily = cfg.TxFamily
	}
	if c.Record.ForkLabel == "" {
		c.Record.ForkLabel = cfg.ForkLabel
	}
	if c.Record.RunStartedAt.IsZero() {
		c.Record.RunStartedAt = time.Now().UTC()
	}
}

func acceptOutcome(ctx context.Context, record TestcaseRecord, rec feedback.Record, sink Sink, stats *Stats) error {
	if rec.CaseID == "" {
		rec.CaseID = record.CaseID
	}
	if rec.SendStatus == "sent" {
		stats.SentCases++
	}
	outcome := Outcome{Case: record, Feedback: rec}
	outcome.Signature = interestingness.SignatureForFeedback(record.TxFamily, record.ForkLabel, rec)
	outcome.Score, outcome.Reasons = interestingness.ScoreFeedback(outcome.Signature, rec)
	result, err := sink.Accept(ctx, outcome)
	if err != nil {
		return err
	}
	if result.Retained {
		stats.RetainedCases++
	}
	if result.Duplicate {
		stats.DuplicateCases++
	}
	return nil
}

type Report struct {
	CampaignID     string    `json:"campaign_id"`
	TxFamily       string    `json:"tx_family"`
	TotalCases     int       `json:"total_cases"`
	SentCases      int       `json:"sent_cases"`
	RetainedCases  int       `json:"retained_cases"`
	DuplicateCases int       `json:"duplicate_cases"`
	GeneratedAt    time.Time `json:"generated_at"`
}

func WriteCaseArtifact(root string, record TestcaseRecord, fb feedback.Record) (string, string, error) {
	casePath, err := WriteCaseMetadataArtifactPath(root, record)
	if err != nil {
		return "", "", err
	}
	feedbackPath, err := WriteFeedbackArtifactPath(root, fb)
	if err != nil {
		return "", "", err
	}
	return casePath, feedbackPath, nil
}

func WriteCaseMetadataArtifact(root string, record TestcaseRecord) error {
	_, err := WriteCaseMetadataArtifactPath(root, record)
	return err
}

func WriteFeedbackArtifact(root string, fb feedback.Record) error {
	_, err := WriteFeedbackArtifactPath(root, fb)
	return err
}

func WriteCaseMetadataArtifactPath(root string, record TestcaseRecord) (string, error) {
	caseDir := filepath.Join(root, "cases")
	if err := os.MkdirAll(caseDir, 0o755); err != nil {
		return "", err
	}
	casePath := filepath.Join(caseDir, record.CaseID+".json")
	if err := writeJSON(casePath, record); err != nil {
		return "", err
	}
	return casePath, nil
}

func WriteFeedbackArtifactPath(root string, fb feedback.Record) (string, error) {
	feedbackDir := filepath.Join(root, "feedback")
	if err := os.MkdirAll(feedbackDir, 0o755); err != nil {
		return "", err
	}
	feedbackPath := filepath.Join(feedbackDir, fb.CaseID+".json")
	if err := writeJSON(feedbackPath, fb); err != nil {
		return "", err
	}
	return feedbackPath, nil
}

func IsCanonicalFeedbackTerminal(fb feedback.Record) bool {
	if fb.ReceiptObserved {
		return true
	}
	if fb.SendStatus != "sent" {
		return true
	}
	for _, flag := range fb.AnomalyFlags {
		if flag == string(ConfirmStateUnresolvedShutdown) {
			return true
		}
	}
	return false
}

func WriteReport(path string, stats Stats) error {
	return writeJSON(path, Report{
		CampaignID:     stats.CampaignID,
		TxFamily:       stats.TxFamily,
		TotalCases:     stats.TotalCases,
		SentCases:      stats.SentCases,
		RetainedCases:  stats.RetainedCases,
		DuplicateCases: stats.DuplicateCases,
		GeneratedAt:    time.Now().UTC(),
	})
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	blob, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o644)
}

func NewCampaignID() string {
	return uuid.NewString()
}

func NewCaseID(sequence int) string {
	return fmt.Sprintf("case-%06d-%s", sequence, uuid.NewString())
}
