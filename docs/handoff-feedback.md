# Handoff And Feedback

## Final Handoff Format

Final responses should include:

- What changed.
- Files changed.
- Verification run.
- Verification not run and why.
- Known risks.
- Follow-up work when relevant.

Keep the handoff concise enough to scan, but include exact command names and outcomes.

## Feedback Loops

When a bug, review comment, failed run, or repeated mistake occurs, convert it into one of:

- A parser fixture.
- A focused Go test.
- A validation script.
- A code comment explaining a non-obvious invariant.
- A user-approved revision to the authoritative docs.

Because `docs/` is authoritative after harness creation, agents must ask before editing docs to record new lessons or revise the harness.
