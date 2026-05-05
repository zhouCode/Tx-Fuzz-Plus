package spammer

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/feedback"
	"github.com/MariusVanDerWijden/tx-fuzz/runner"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type testCampaignEvent struct {
	CaseID          string   `json:"case_id"`
	CampaignID      string   `json:"campaign_id"`
	Stage           string   `json:"stage"`
	SendStatus      string   `json:"send_status"`
	ReceiptObserved bool     `json:"receipt_observed"`
	AnomalyFlags    []string `json:"anomaly_flags,omitempty"`
}

func TestCampaignSubmitterSubmitAsyncReturnsBeforeFinalize(t *testing.T) {
	submitter := &campaignSubmitter{
		rpcLabel:       "rpc-a",
		receiptTimeout: time.Second,
		sendTx: func(context.Context, *types.Transaction) error {
			return nil
		},
	}
	c := &runner.Case{Record: runner.TestcaseRecord{CaseID: "case-1"}, Tx: types.NewTx(&types.LegacyTx{})}

	pending, err := submitter.SubmitAsync(context.Background(), c)
	if err != nil {
		t.Fatalf("submit async: %v", err)
	}
	record, err := pending.Await(context.Background())
	if err != nil {
		t.Fatalf("await returned error: %v", err)
	}
	if record.SendStatus != "sent" || record.ReceiptObserved {
		t.Fatalf("unexpected async submit record: %#v", record)
	}
}

func TestCampaignSubmitterSubmitLegacyWritesEventsJSONL(t *testing.T) {
	ready := make(chan struct{})
	release := make(chan struct{})
	root := t.TempDir()
	submitter := &campaignSubmitter{
		rpcLabel:       "rpc-a",
		receiptTimeout: time.Second,
		artifactRoot:   root,
		sendTx: func(context.Context, *types.Transaction) error {
			return nil
		},
		waitMined: func(context.Context, *ethclient.Client, *types.Transaction) (*types.Receipt, error) {
			close(ready)
			<-release
			return &types.Receipt{Status: types.ReceiptStatusSuccessful, GasUsed: 21000}, nil
		},
	}

	eventsPath := filepath.Join(root, "events.jsonl")
	recordCh := make(chan feedback.Record, 1)
	errCh := make(chan error, 1)
	go func() {
		record, err := submitter.Submit(context.Background(), &runner.Case{
			Record: runner.TestcaseRecord{CaseID: "case-events", CampaignID: "campaign-events"},
			Tx:     types.NewTx(&types.LegacyTx{}),
		})
		if err != nil {
			errCh <- err
			return
		}
		recordCh <- record
	}()

	select {
	case <-ready:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected waitMined to start")
	}

	events := waitForCampaignEvents(t, eventsPath, 1)
	if events[0].Stage != "submitted" || events[0].SendStatus != "sent" || events[0].ReceiptObserved {
		t.Fatalf("unexpected submitted event: %#v", events[0])
	}

	close(release)

	var record feedback.Record
	select {
	case err := <-errCh:
		t.Fatalf("submit: %v", err)
	case record = <-recordCh:
	case <-time.After(time.Second):
		t.Fatal("submit did not finalize")
	}
	if !record.ReceiptObserved {
		t.Fatalf("expected finalized receipt: %#v", record)
	}

	events = waitForCampaignEvents(t, eventsPath, 2)
	if events[1].Stage != "finalized" || !events[1].ReceiptObserved || events[1].SendStatus != "sent" {
		t.Fatalf("unexpected finalized event: %#v", events[1])
	}
}

func TestCampaignSubmitterAwaitReceiptAnnotatesReceiptTimeout(t *testing.T) {
	submitter := &campaignSubmitter{
		rpcLabel:       "rpc-a",
		receiptTimeout: 10 * time.Millisecond,
		waitMined: func(ctx context.Context, _ *ethclient.Client, _ *types.Transaction) (*types.Receipt, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	c := &runner.Case{Record: runner.TestcaseRecord{CaseID: "case-timeout"}, Tx: types.NewTx(&types.LegacyTx{})}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	record, err := submitter.AwaitReceipt(ctx, c)
	if err != nil {
		t.Fatalf("await receipt: %v", err)
	}
	if !slices.Contains(record.AnomalyFlags, "receipt_timeout") {
		t.Fatalf("missing receipt_timeout anomaly: %#v", record)
	}
	if record.ReceiptObserved {
		t.Fatalf("timeout should not mark receipt observed: %#v", record)
	}
}

func TestCampaignSubmitterSubmitAsyncAllowsNextSubmitBeforeFirstAwait(t *testing.T) {
	firstTx := types.NewTx(&types.LegacyTx{Nonce: 1})
	secondTx := types.NewTx(&types.LegacyTx{Nonce: 2})
	submitter := &campaignSubmitter{
		rpcLabel:       "rpc-a",
		receiptTimeout: time.Second,
		sendTx: func(context.Context, *types.Transaction) error {
			return nil
		},
	}

	firstPending, err := submitter.SubmitAsync(context.Background(), &runner.Case{
		Record: runner.TestcaseRecord{CaseID: "case-1"},
		Tx:     firstTx,
	})
	if err != nil {
		t.Fatalf("submit first: %v", err)
	}

	secondPendingCh := make(chan runner.PendingSubmission, 1)
	secondErrCh := make(chan error, 1)
	go func() {
		pending, err := submitter.SubmitAsync(context.Background(), &runner.Case{
			Record: runner.TestcaseRecord{CaseID: "case-2"},
			Tx:     secondTx,
		})
		if err != nil {
			secondErrCh <- err
			return
		}
		secondPendingCh <- pending
	}()

	var secondPending runner.PendingSubmission
	select {
	case err := <-secondErrCh:
		t.Fatalf("submit second: %v", err)
	case secondPending = <-secondPendingCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second submit should return before any receipt wait")
	}
	if record, err := firstPending.Await(context.Background()); err != nil || record.SendStatus != "sent" {
		t.Fatalf("await first: record=%#v err=%v", record, err)
	}
	if record, err := secondPending.Await(context.Background()); err != nil || record.SendStatus != "sent" {
		t.Fatalf("await second: record=%#v err=%v", record, err)
	}
}

func TestCampaignSubmitterAwaitReceiptPropagatesLookupFailureAsRecord(t *testing.T) {
	submitter := &campaignSubmitter{
		rpcLabel:       "rpc-a",
		receiptTimeout: time.Second,
		waitMined: func(context.Context, *ethclient.Client, *types.Transaction) (*types.Receipt, error) {
			return nil, errors.New("lookup boom")
		},
	}
	record, err := submitter.AwaitReceipt(context.Background(), &runner.Case{
		Record: runner.TestcaseRecord{CaseID: "case-lookup"},
		Tx:     types.NewTx(&types.LegacyTx{}),
	})
	if err != nil {
		t.Fatalf("await receipt: %v", err)
	}
	if !slices.Contains(record.AnomalyFlags, "receipt_lookup_failed") {
		t.Fatalf("missing lookup anomaly: %#v", record)
	}
	if len(record.ProcessSignals.StderrHints) == 0 {
		t.Fatalf("missing lookup stderr hint: %#v", record)
	}
}

func waitForCampaignEvents(t *testing.T, path string, want int) []testCampaignEvent {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		events, err := readCampaignEvents(path)
		if err == nil && len(events) >= want {
			return events
		}
		time.Sleep(10 * time.Millisecond)
	}
	events, err := readCampaignEvents(path)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	return events
}

func readCampaignEvents(path string) ([]testCampaignEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []testCampaignEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event testCampaignEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
