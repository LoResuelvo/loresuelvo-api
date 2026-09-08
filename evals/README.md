# US-60 evaluation integration

Status: dataset imported and checked; Go runner not implemented. No live model
evaluation has been performed.

## Source and integrity

- Specification: https://github.com/LoResuelvo/loresuelvo-api/issues/205
- Original archive: `incoming/LoResuelvo_US60_evals_v1.0.0.zip` (local, ignored).
- Archive SHA-256: `f253e81a18af481f0b91fad3222ac3c7f6de1e9d985c9c8a971eadfcaf276808`.
- Unmodified package: `datasets/LoResuelvo_US60_evals_v1.0.0/`.
- Reference backend commit: `2a7f77fda6c6175753f73e095de0a0bd19169a06`.
- Import commit: pending; no commit has been created for this import.

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

## Delivery checkpoints

Each increment is reviewed and tested before its commit and push. Planned
commit boundaries (split further only when an independently testable change
justifies it):

1. `chore(us-60): import immutable evaluation dataset`
2. `feat(us-60): add offline dataset mapping and execution preview`
3. `feat(us-60): execute contracts and persist evaluation results`
4. `feat(us-60): add bounded opt-in live evaluation`
5. `feat(us-60): score and compare reproducible evaluation runs`
6. `docs(us-60): record baseline evidence and evaluation limitations`

Use focused tests during development, then the existing CI-equivalent checks
before pushing Go changes. Live calls are never part of those checks. Review
and retain the unchanged baseline before any functional correction. Baseline
execution still requires explicit model configuration, limits, and live opt-in.
