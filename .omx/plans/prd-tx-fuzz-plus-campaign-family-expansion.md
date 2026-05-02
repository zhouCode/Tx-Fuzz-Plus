# Tx-Fuzz++ Campaign Family Expansion PRD

> Source context: `.omx/context/tx-fuzz-plus-unsupported-campaign-families-20260502T153442Z.md`
> Brownfield source-of-truth: existing Phase 1 artifacts in `.omx/specs/deep-interview-tx-fuzz-plus-upgrade-plan.md`, `.omx/plans/prd-tx-fuzz-plus-upgrade-plan.md`, and `.omx/plans/test-spec-tx-fuzz-plus-upgrade-plan.md`

## Requirements Summary

Extend the existing `campaign basic` workflow to **all currently unsupported campaign transaction families** that are already present elsewhere in the repo. Based on the current codebase, that means:
- `blob` transactions via `RandomBlobTx(...)` (`transactions.go:118-125`)
- `auth/7702` transactions via `RandomAuthTx(...)` (`transactions.go:127-143`)

This PRD snapshots the unsupported family set as of 2026-05-02. If another unsupported campaign family appears before implementation starts, treat it as a follow-up planning item rather than silently widening this increment.

Out of scope for this plan:
- legacy / 1559 / access-list split as separate campaign families, because those are already covered by `RandomValidTx(...)` and the current `campaign basic` family (`transactions.go:106-116`, `spammer/campaign.go:123-156`, `cmd/livefuzzer/main.go:51-61`)
- phase-2 spec-guided generation
- multi-client differential orchestration as a first-class automated mode

## Current Brownfield Evidence

1. The current campaign CLI only exposes `campaign basic` in `cmd/livefuzzer/main.go:51-61`.
2. `campaign basic` is implemented through a single family-specific path in `spammer/campaign.go:43-104` and `spammer/campaign.go:106-217`.
3. Blob and 7702 send loops already exist outside campaign mode:
   - `spammer/blob.go:19-59`
   - `spammer/eip7702.go:21-78`
4. The campaign core (artifact write, scoring, retention, replay) already exists and is family-agnostic enough to reuse:
   - `runner.RunCampaign(...)` in `runner/types.go`
   - signature/scoring in `interestingness/`
   - retention in `corpus/`
   - replay bundle export in `replay/`
5. Endpoint selection is already dynamic via `spammer/endpoints.go` and is used by campaign/replay/config flows.

## RALPLAN-DR Summary

### Principles
1. **Reuse existing tx constructors** rather than reimplementing transaction-family semantics.
2. **Keep campaign lifecycle generic**; isolate family differences to builder/signing/metadata/normalization hooks.
3. **Preserve replayability** as a first-class requirement for every newly supported family.
4. **Prefer CLI consistency with existing user-facing naming** when adding campaign subcommands.
5. **Validate against dynamic devnet endpoints** resolved from `~/ethpackage/endpoints.json`, never hardcoded ports.

### Decision Drivers
1. Minimize brownfield risk by reusing `RandomBlobTx(...)` and `RandomAuthTx(...)`.
2. Avoid fragmenting the campaign pipeline into duplicated per-family implementations.
3. Preserve clear user ergonomics and artifact comparability across families.

### Viable Options

#### Option A — Add family-specific adapters on top of the current campaign core (**Recommended**)
- Add `campaign blob` and `campaign pectra` subcommands.
- Do **not** add a `campaign auth` alias in this increment unless this PRD is explicitly amended.
- Reuse the existing runner/sink/report pipeline.
- Extract only the family-specific builder/signing/metadata pieces.
- Pros: smallest diff, highest reuse, easiest to verify incrementally.
- Cons: requires modest refactor of `spammer/campaign.go` to avoid hardcoding `basic` assumptions.

#### Option B — Introduce a fully generic family registry before adding any new family
- Create a registry/config model for every campaign family first.
- Pros: theoretically clean extensibility.
- Cons: higher abstraction cost now, slows delivery, risks overdesign before blob/auth parity is proven.

#### Option C — Duplicate `campaign basic` into separate `campaignBlob` / `campaignAuth` implementations
- Pros: fastest initial coding path.
- Cons: duplicates lifecycle logic, increases maintenance burden, weakens consistency of artifacts and replay behavior.

### Alternative Invalidation Rationale
Option C is rejected because duplication would immediately diverge retention/replay/report semantics across families. Option B is rejected for now because the current repo only has two unsupported families left, so a lighter adapter-based structure is enough.

## Architect Review Summary

### Steelman Antithesis
A strict architect could argue that adding blob and 7702 on top of the current `spammer/campaign.go` risks baking family quirks into one file and creating a future refactor burden. If more tx families arrive, a stronger registry abstraction may become unavoidable.

### Real Tradeoff Tension
- **Too little abstraction:** `spammer/campaign.go` grows into a family switchboard.
- **Too much abstraction too early:** implementation slows and basic/blob/auth parity gets blocked on framework design.

### Synthesis
Use a **thin family adapter layer now**:
- common campaign lifecycle remains shared
- one small builder/signing/metadata adapter per family
- factor shared helper functions out of `spammer/campaign.go` only when blob/auth make the duplication obvious

## Critic Verdict
**APPROVE WITH INCORPORATED IMPROVEMENTS**

Applied improvements in this final plan:
- Explicitly scoped “all unsupported transaction types” to `blob` and `auth/7702`
- Preserved evidence-backed exclusion of legacy/1559/access-list as already covered under `basic`
- Added replay/data-model requirements per family rather than only CLI additions
- Added verification matrix and launch/staffing guidance for later execution handoff

## ADR

### Decision
Implement unsupported campaign families in **two staged increments** on top of the existing campaign core:
1. `campaign blob`
2. `campaign pectra` (user-facing name aligned with current CLI) backed by auth/7702 semantics

### Drivers
- Existing tx builders already exist for blob/auth families.
- Existing campaign pipeline is already usable for shared lifecycle steps.
- User-facing CLI already uses `pectra` for 7702 spam, so preserving that naming reduces surprise.

### Alternatives considered
- Separate `campaign auth` command name or alias in the same increment
- Full family registry refactor before feature expansion
- Per-family duplicated implementations

### Why chosen
This balances user-facing consistency (`pectra`) with internal semantic clarity (metadata and normalization can still track auth/7702 details). It avoids blocking delivery on a larger framework rewrite.

### Consequences
- Campaign metadata/reporting should distinguish **CLI family label** from **protocol semantics** where needed.
- `spammer/campaign.go` likely needs modest refactoring to split common flow from family-specific adapters.
- Blob/auth error normalization must be extended before smoke validation is trustworthy.

### Follow-ups
- Consider a more formal family registry only if additional tx families or templated spec-guided families are added later.
- Revisit whether a `campaign auth` alias should be added only after `campaign pectra` lands and parity is proven.

## Proposed Scope

### Family 1: `campaign blob`
Add a bounded campaign path for blob transactions that:
- uses `RandomBlobTx(...)`
- signs with Cancun signer semantics
- records blob-specific metadata
- produces blob-aware replay bundles
- classifies blob-specific error modes

### Family 2: `campaign pectra`
Add a bounded campaign path for 7702/auth transactions that:
- uses `campaign pectra` as the required user-facing command name for this increment
- does not introduce `campaign auth` as a compatibility alias in the same change set
- reuses the authorization creation pattern from `spammer/eip7702.go:37-54`
- uses `RandomAuthTx(...)`
- signs with Prague signer semantics
- records authorization-specific metadata
- extends replay and signature classification for auth-specific failures

## File / Module Plan

### Modify
- `cmd/livefuzzer/main.go`
  - add `campaign blob`
  - add `campaign pectra`
- `flags/flags.go`
  - add any family-specific flags only if absolutely needed
- `spammer/campaign.go`
  - refactor current basic-specific flow into shared helpers + family adapters
- `interestingness/signature.go`
  - extend normalization for blob/auth-specific classes
- `replay/export.go`
  - confirm bundle metadata/replay commands remain family-complete
- `runner/types.go`
  - extend testcase metadata if current fields are insufficient

### Add tests
- `spammer/campaign_blob_test.go`
- `spammer/campaign_pectra_test.go`
- extend `cmd/livefuzzer/main_test.go`
- extend `interestingness/signature_test.go`
- extend `runner/types_test.go`
- extend `replay/export_test.go`

## Data Model Changes

### Required compatibility rules
- Preserve backward compatibility for existing `campaign basic` case/report readers: additive fields are allowed, but existing field meanings must not be redefined.
- Distinguish **CLI family label** from **protocol transaction semantics**. If the current schema cannot express both safely, add explicit fields rather than overloading one existing family field.
- A newly added family is not complete unless its serialized testcase/report metadata is sufficient for replay/export, signature normalization, and report comparison without consulting ad-hoc side channels.

### Blob additions
Extend testcase metadata/reporting with at least these fields or equivalent existing schema slots with the same semantics:
- protocol family marker for blob cases (for example `TxFamily = "blob"`)
- `BlobCount`
- blob-related fee fields in `FeeFields`
- blob payload size / calldata-size signal when cheaply derivable

### 7702 / auth additions
Extend testcase metadata/reporting with at least these fields or equivalent existing schema slots with the same semantics:
- protocol family marker for auth/7702 cases
- `AuthorizationCount`
- CLI/report label needed to distinguish `campaign pectra` from the underlying auth/7702 protocol semantics when both appear in artifacts
- optional authorization signer/address hints when useful for replay/debugging
- auth-specific failure classes in feedback/signature normalization

## Implementation Steps

### Step 1 — Refactor the current campaign path into reusable family adapters
- Extract current `basicCampaignBuilder`/submitter assumptions into a reusable family adapter shape.
- Keep `runner.RunCampaign(...)` untouched unless a real gap is found.
- Preserve current `campaign basic` behavior while refactoring.

### Step 2 — Add `campaign blob`
- CLI: add `campaign blob` subcommand beside `campaign basic`.
- Builder: create blob tx using `RandomBlobTx(...)`.
- Signer: keep Cancun signer.
- Metadata: set `TxFamily = "blob"`, populate blob-specific fields.
- Replay: a blob replay bundle is complete only if it can be resubmitted by the existing replay flow without reopening the original corpus artifact; at minimum preserve the signed raw tx, chain/environment context, blob-count signal, and any blob sidecar/hash material required by the current replay/export path.
- Reporting: ensure blob campaign reports remain structurally identical to basic.

### Step 3 — Add blob-specific normalization and tests
- Add blob-specific error classification buckets instead of collapsing everything into generic `rpc_error`.
- Add tests for metadata round-trip, replay bundle completeness, and CLI registration.

### Step 4 — Add `campaign pectra`
- CLI: add `campaign pectra` subcommand to match current user-facing naming.
- Builder: reuse auth creation logic from `spammer/eip7702.go` and `RandomAuthTx(...)`.
- Signer: Prague signer.
- Metadata: set protocol-aware family fields and authorization count.

### Step 5 — Add 7702/auth normalization and tests
- Add auth-specific failure classes and signature buckets.
- Ensure replay bundles include enough information to reproduce 7702/auth submissions without consulting the original testcase artifact; at minimum preserve the signed raw tx, chain/environment context, authorization count, and any serialized authorization payload or derived fields required by the current replay/export path.
- Add CLI, metadata, replay, and retention tests.

### Step 6 — Smoke validation by family
For each newly added family:
- resolve endpoint dynamically from `~/ethpackage/endpoints.json`
- fail fast with a clear error if the endpoints file is missing, malformed, stale, unreachable, ambiguous for the selected `el_client`, or incompatible with the chosen family; do not fall back to hardcoded ports
- run a small bounded campaign against one selected execution node
- verify case artifacts, retained findings, replay bundle, and report JSON exist and are coherent

## Risks and Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Blob/auth-specific errors collapse into generic RPC buckets | Low-quality retained findings | Extend `interestingness/signature.go` before declaring family support complete |
| 7702 auth generation needs extra signing context beyond basic builder shape | Campaign path may become awkward | Reuse the proven auth creation logic from `spammer/eip7702.go` instead of inventing new semantics |
| Replay bundles may be insufficient for blob/auth reproduction | Retained findings lose value | Add explicit replay completeness tests per family |
| `spammer/campaign.go` grows too large | Maintainability drops | Extract family adapter helpers when duplication appears during blob/auth implementation |

## Acceptance Criteria

1. `cmd/livefuzzer` exposes `campaign basic`, `campaign blob`, and `campaign pectra`.
2. `campaign basic` behavior remains green after refactor, including CLI help/registration, artifact categories, replay export shape, and retained-finding semantics.
3. `campaign blob` produces artifacts/replay/report with family-correct metadata.
4. `campaign pectra` produces artifacts/replay/report with auth-correct metadata.
5. Blob/auth-specific signature normalization avoids collapsing all failures into generic buckets.
6. Smoke runs for each added family resolve RPC dynamically from `~/ethpackage/endpoints.json`.
7. Endpoint-resolution failures for the new family smokes are explicit and deterministic rather than silently using fallback ports or whichever node happens to respond first.

## Verification Steps

- `go build ./...`
- `go test ./...`
- targeted tests for campaign family adapters and replay/signature changes
- smoke run for `campaign blob`
- smoke run for `campaign pectra`
- replay smoke for at least one retained case from each family
- regression/parity check that `campaign basic` CLI output and artifact layout remain unchanged except for additive family support

## Available-Agent-Types Roster

From the known repo/session guidance, the practical roster for follow-up execution is:
- `explore` — repo lookup and mapping
- `executor` — implementation lane
- `debugger` — failure triage lane
- `test-engineer` / `verifier` — regression and completion evidence lane
- `critic` / `architect` — final plan/architecture review lane

## Follow-up Staffing Guidance

### For `$ralph`
- Lane 1: `executor` (medium/high reasoning) — refactor campaign core and add blob/pectra support
- Lane 2: `test-engineer` or `verifier` (medium/high reasoning) — add family-specific tests and smoke validation
- Lane 3: `debugger` (high reasoning, on demand) — investigate replay/signature failures if smoke runs expose family-specific issues

### For `$team`
- Worker A: blob family implementation
- Worker B: pectra family implementation
- Worker C: shared signature/replay/report updates and regression verification
- Leader verifies CLI/report consistency and merges shared-file changes carefully

## Launch Hints

### Ralph
```text
$ralph .omx/plans/prd-tx-fuzz-plus-campaign-family-expansion.md
```

### Team
```text
$team .omx/plans/prd-tx-fuzz-plus-campaign-family-expansion.md
```

## Team Verification Path

Before team shutdown:
1. both new campaign subcommands exist and build
2. shared tests are green
3. at least one smoke run per family completed against a dynamically resolved endpoint
4. retained artifacts + replay bundles exist for blob and pectra families
5. final Ralph/verifier pass confirms no regression to `campaign basic`
