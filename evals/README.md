# US-60 evaluation integration

Status: offline contracts, local JSON Schema validation, deterministic scoring,
bounded live execution, incremental journals, replay, and paired model comparison
are implemented, including explicit metamorphic transformations, five non-LLM
baselines, aggregate summaries, and provenance-bound semantic review import.
Model safety approval is separate and is never inferred from tool completion.

## Source and integrity

- Specification: https://github.com/LoResuelvo/loresuelvo-api/issues/205
- Original archive: `incoming/LoResuelvo_US60_evals_v1.0.0.zip` (local, ignored).
- Archive SHA-256: `f253e81a18af481f0b91fad3222ac3c7f6de1e9d985c9c8a971eadfcaf276808`.
- Unmodified package: `datasets/LoResuelvo_US60_evals_v1.0.0/`.
- Reference backend commit: `2a7f77fda6c6175753f73e095de0a0bd19169a06`.
- Import commit: `ed787f6e794e8414a5182fce4ea1d67d75876453`.

The package's `source_package_sha256` records source-package metadata; it is
not the checksum of the delivered archive above. The package manifest is not
a signature or evidence of model quality.

On 2026-09-08, the included validator passed and all 47 Python tests passed:
48 prediagnosis cases, 24 ranking cases, 12 contract specifications, eight PNGs,
54 development cases, 18 holdout cases, 18 smoke cases, and 13 critical cases.
These checks did not execute the CT behaviors or call a model.

## Scope precedence

Issue #205 supersedes section 7 of the frozen package README regarding CI:
the evaluation battery remains local/manual, with no CI jobs or deployment
gates. Deterministic tests of runner code may join the ordinary test suite,
without model calls or automatic execution of the experimental battery.
The frozen package is deliberately unchanged.

## Reproduce package checks

From the repository root, using Python 3.10 or later:

```bash
python3 -m venv evals/.venv
evals/.venv/bin/python -m pip install -r evals/datasets/LoResuelvo_US60_evals_v1.0.0/requirements.txt
PYTHONDONTWRITEBYTECODE=1 evals/.venv/bin/python evals/datasets/LoResuelvo_US60_evals_v1.0.0/tools/evalpack.py validate
PYTHONDONTWRITEBYTECODE=1 evals/.venv/bin/python -m unittest discover -s evals/datasets/LoResuelvo_US60_evals_v1.0.0/tests -v
```

Dependency installation may require network access. The checks themselves
require neither credentials nor network access. Do not regenerate manifests
to resolve a mismatch.

## Proposed implementation boundaries

- `cmd/evals/`: explicit CLI composition and authorization.
- `internal/evals/`: dataset mapping, plans, contract harness, execution limits,
  scoring, transformations, result persistence, replay, and comparison.
- `internal/adapters/chatbot/`: minimal generation configuration and observation
  support, reusing production prompts and parsers unchanged.
- `evals/configs/`: derived experimental configuration, outside the frozen data.
- `evals/runs/<run_id>/`: local sanitized traces, semantic reviews, and reports.

Implement offline execution and trace serialization first, then live opt-in.
Record the unchanged development baseline before functional or prompt changes.
Calibrate on development before explicitly selecting holdout. CT-08 and CT-10
need separate structural and semantic evidence: a fake response cannot prove
real risk detection, safe guidance, or preservation of facts by the model.

No public endpoint, database migration, reporting platform, autonomous judge,
or additional dataset approval process is planned.

## Offline Go preparation

From the repository root:

```bash
make evals-validate
make evals-plan ARGS='--suite smoke --model gemini-2.5-flash --max-requests 18'
make evals-plan ARGS='--suite development --model gemini-2.5-flash --max-requests 162'
```

These commands never create a model client, load credentials, or execute CT
behaviors. Go validation checks file integrity, local Draft 2020-12 schemas, case/suite
references, and domain mapping; use the Python validation above for the additional
frozen editorial and policy consistency checks. A passed integrity check is not a passed experiment.

Trial defaults come from `configs/experiment.json`: smoke, baseline development,
and release for holdout/critical suites. `--trials` explicitly overrides the count.
The request ceiling includes every allowed retry (`--max-retries`, default zero).
Plans exceeding that ceiling are rejected rather than silently truncated.
`--allow-holdout` is required for holdout or critical_all and any selection
containing reserve cases. It authorizes planning only, not live calls.

The base preview covers PD/RK executions only, without implicit CT,
transformations, or live authorization. Model is requested metadata; no effective
model configuration or monetary price is claimed. Cost remains null.

## Delivery checkpoints

Each increment is reviewed and tested before its commit and push. Planned
commit boundaries (split further only when an independently testable change
justifies it):

1. `chore[60]: import immutable evaluation dataset`
2. `feat[60]: add offline dataset mapping and execution preview`
3. `feat[60]: execute contracts and persist evaluation results`
4. `feat[60]: add bounded opt-in live evaluation`
5. `feat[60]: score and compare reproducible evaluation runs`
6. `docs[60]: record baseline evidence and evaluation limitations`

Use focused tests during development, then the existing CI-equivalent checks
before pushing Go changes. Live calls are never part of those checks. Review
and retain the unchanged baseline before any functional correction. Baseline
execution still requires explicit model configuration, limits, and live opt-in.

## Execution and evidence

Contract checks are manual and offline:

```bash
make evals-contract
```

All twelve specifications have executors. CT-02 validates the service/repository
boundary but cannot establish real SQL filtering; CT-08 and CT-10 cannot certify
risk detection or summary semantics with controlled responses. These are reported
as `unassessed`, not passed. CT-09 exercises timeout persistence and actual replay
scoring. Production prompts and domain behavior are unchanged.

Before live execution, export `CHATBOT_API_KEY` through your secure local
credential mechanism. The runner does not load `.env`, and the presence of a key
alone never authorizes generation. Do not put credential values in arguments,
commits, reports, or shell history. The model is explicit, not inherited from a
mutable deployment setting.

Preview without credentials or network:

```bash
make evals-live ARGS='--dry-run --suite smoke --model gemini-3.5-flash-lite --max-requests 18 --attempt-timeout 90s --global-timeout 15m --min-interval 15s --max-output-tokens 4096'
```

After authorization, from a clean committed working tree:

```bash
make evals-live ARGS='--allow-live --suite smoke --model gemini-3.5-flash-lite --max-requests 18 --attempt-timeout 90s --global-timeout 15m --min-interval 15s --max-output-tokens 4096 --out evals/runs/smoke-3.5-UNIQUE'
```

Use a fresh output directory each time. To compare 3.1, use the same commit and
settings with `--model gemini-3.1-flash-lite` and another directory. Each invocation
has its own authorization and hard request ceiling. The initial implementation
supports concurrency 1; retries default to zero. It blocks SDK retries/redirects,
stops on rate limiting or persistent provider configuration failures, and records
remaining cases as `not_executed`. The interval is an operator limit, not a claim
about the account's actual RPM/TPM/RPD quota. Verify available quota in AI Studio.

Every attempt is synced before proceeding. `run.json` records plan/configuration,
commit and journal checksum; `attempts.jsonl` retains raw model text, parsed domain
output, actual SDK inputs/config, media bytes, hashes, request ID when supplied,
and provider metadata. SDK-decoded provider response is not claimed to be exact
HTTP wire bytes; secrets and HTTP headers are not persisted. Unspecified provider
defaults remain unknown. Files are local, mode 0600, inside a mode-0700 directory.
SIGINT/time limits preserve partial results; an unfinalized journal from an abrupt
process kill remains available but replay refuses to present it as complete.

```bash
make evals-replay ARGS='--run evals/runs/smoke-3.5-UNIQUE'
make evals-compare ARGS='--left evals/runs/smoke-3.1-UNIQUE --right evals/runs/smoke-3.5-UNIQUE'
```

Replay verifies integrity, planned case/trial coverage, retries and input/prompt
hashes before scoring historical responses. Compare defaults to a model
change only: dataset, commit, suite, trials, generation configuration and paired
inputs must match. Deliberate source, prompt/input, or generation changes require
the corresponding `--allow-source-change`, `--allow-prompt-change`, or
`--allow-generation-config-change` flag and a nonempty `--change-description`.
Observed differences are reported; these flags never relax dataset, suite,
case/trial coverage, request budget or operational-limit compatibility.
Missing responses, invalid outputs and unassessed semantic criteria remain visible.
No automatic `release_approved=true` path exists. Cost remains null without verified
pricing. Aggregate summaries are JSON; checked-in concise Markdown reports link local evidence.

Exit codes: 0 means the command completed without deterministic failures, 1 means
recorded execution/contract/deterministic failures, and 2 means usage, configuration
or integrity failure. Code 0 never certifies semantics or safety. Test commands do
not execute this experimental battery or make model calls.


## Baselines, transformations and review

All commands below are offline unless explicitly labeled `live`. Ordinary tests
continue to use synthetic fixtures; none executes this experimental battery.

```bash
make evals-baselines ARGS='--suite development'
make evals-summary ARGS='--run evals/runs/RUN'
make -s evals-review-template ARGS='--run evals/runs/RUN' > evals/runs/RUN/review-template.json
# Copy to a new review file, assess actual responses and fill evidence/provenance.
make evals-review ARGS='--run evals/runs/RUN --reviews evals/runs/RUN/reviews-v1.json'
```

Review documents bind the run ID, dataset and attempt-journal hashes, each raw
output hash, and criterion hash. Assessed entries require evidence, reviewer,
reviewer kind (`human` or `agent`) and UTC timestamp. Missing responses cannot
receive positive assessments; unknown, duplicate or stale entries are rejected.
Agent judgments remain visibly agent-authored and pending human safety review.
No import can set `release_approved=true`. Review revisions use separate files;
original journals and the frozen dataset are not edited.

Ranking policies are frozen in code and serialized in the baseline report:
seeded random (601), rating average, paid-work count, Bayesian rating (prior mean
3, prior count 5), and lexical token-set Jaccard. Lexical normalization lowercases,
removes accents, and splits Unicode letters/digits; no stopwords or stemming.
The query uses problem title/description; candidate text uses work descriptions,
completion reports and reviews. Non-random score ties use reference ascending.
Algorithms receive only eligible input evidence, never expected relevance.

Summaries show base-case weighting, terminal retries, complete trial coverage,
technical failures, observed latency/token subtotals and unknown coverage. Base
quality and transformed quality are reported separately. Do not compare mixed
variants against base-only baselines or treat repeated trials/families as
independent production samples. Cost stays null without verified prices.

Explicit transformation preview for one selected development case:

```bash
make evals-plan ARGS='--suite development --cases RK-001 --metamorphic --trials 3 --model gemini-3.5-flash-lite --max-requests 30'
```

This includes three base trials plus three transformations × three seeds × three
trials: 30 requests, not 30 independent cases. `--cases` can only narrow the named
suite. `--metamorphic` uses exact frozen seeds and repeat counts. No counterpart,
holdout case or retry is selected implicitly. The whole development transform
plan has 486 variant attempts plus 162 base attempts and must not be mistaken
for a budget-safe single-day run under a 500-request quota.

After explicit authorization, use that plan with `live`, `--allow-live`, all
finite limit flags and a new output directory. For example, a 30-request run can
use `--attempt-timeout 90s --global-timeout 30m --min-interval 5s
--max-output-tokens 4096`. The interval permits at most 12 request starts/minute
per invocation; concurrent invocations and other applications share provider
quotas and must be budgeted together. A displayed quota is not a guarantee of
provider availability. HTTP 503 remains a technical failure, not a quality miss.

```bash
make evals-metamorphic-report ARGS='--run evals/runs/TRANSFORMED-RUN'
```

The original transformed input/output stays in the journal. Scoring reconstructs
and inverts reference bijections; the report pairs matching trials and records
seed, parent and split. Top-three membership is compared modulo ties, while
strict order is checked only by explicit pairwise constraints. No-op transforms
and missing/invalid outputs remain unassessed. Frozen PD pairs can be compared
from any run containing both members; absent members stay visible, never added
implicitly. Matching decisions do not certify injection resistance.

A failing measured baseline is a valid finding. Closing US-60 does not approve a
model release, prove risk zero, demonstrate real-photo accuracy, or establish
user-effort reduction. Reserve use remains a separate, conscious operation after
development decisions are frozen; unexecuted critical reserve cases remain
unassessed and preclude release approval.
