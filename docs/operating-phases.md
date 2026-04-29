# Operating Phases

Agents working in this repo should follow these phases. Low-risk changes can compress the narration, but not the gates.

## 1. Intake

Restate the requested change, identify affected areas, and note any blockers or assumptions.

## 2. Discovery

Read `AGENTS.md` and the relevant docs for the touched area. Inspect existing code, tests, fixtures, and manifests before planning edits.

## 3. Plan

For non-trivial work, state the files or packages likely to change, the verification plan, and the main risks.

## 4. Implementation

Make scoped edits. Keep parsing, watching, aggregation, and TUI rendering separated by package boundaries.

## 5. Verification

Run required checks from [verification.md](verification.md). Capture command names and outcomes. If a required check cannot run, explain why and describe residual risk.

## 6. Handoff

Report changed files, verification, skipped checks, known risks, and follow-up work.
