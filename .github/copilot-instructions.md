# Copilot Instructions

## Purpose

This repository uses disciplined, maintainable, production-grade software engineering practices. These instructions apply to all work in this repository and override broader guidance when they are more specific.

The goal is to prevent low-quality generated code by requiring structured engineering workflow, architectural consistency, testability, scope control, security awareness, and long-term maintainability.

## Engineering Priorities

When making tradeoffs, prioritize:

1. Correctness: satisfy the stated behavior, handle edge cases, and avoid regressions.
2. Readability: prefer clear names, direct control flow, and code a maintainer can scan quickly.
3. Maintainability: keep changes scoped, cohesive, and aligned with existing patterns.
4. Testability: design boundaries so behavior can be verified with deterministic tests.
5. Modularity: separate responsibilities and preserve clear ownership boundaries.
6. Security: validate trust boundaries and avoid exposing sensitive data.
7. Observability: make important failures diagnosable with useful errors and logging.
8. Documentation: keep public contracts, setup notes, and architectural rationale current.
9. Low complexity: favor small functions, shallow branching, and simple data flow.
10. Incremental validation: make changes in reviewable steps and run relevant checks as work progresses.

## Repository Context


## Required Scope Discipline

Before changing code:

- Understand the requirement and expected behavior.
- Identify the affected areas and existing tests.
- Identify what should remain unchanged.
- Inspect existing conventions, dependencies, and architecture before introducing new patterns.
- Keep the change limited to the requested scope.
- Do not refactor unrelated code.
- Do not rename unrelated symbols.
- Do not reformat untouched files.
- Do not change behavior outside the requested scope unless required for correctness, compatibility, or safety.

If a broader change appears necessary, explain why before proceeding.

## Development Workflow

For non-trivial code changes, follow this sequence:

1. Understand the requirement and define expected behavior.
2. Define scope boundaries and identify what should remain unchanged.
3. Identify key risks, dependencies, edge cases, and validation needs.
4. Create concise implementation pseudocode covering architecture, data flow, dependency interactions, error handling, and testability.
5. Define or update tests before implementation; for bug fixes, include a regression test when practical.
6. Create or update the tests and, when practical, verify they fail for the intended reason before changing production code.
7. Implement the smallest incremental change needed to satisfy the tests and scope.
8. Run the relevant validation checks and verify tests pass.
9. Review complexity, coupling, observability, documentation needs, and maintainability.
10. Update documentation when behavior, setup, public contracts, or architecture change.

Implementation should not begin before scope is clear, pseudocode is written, and tests are defined for non-trivial behavior changes. Implementation should align with the pseudocode. If implementation reveals a need to deviate significantly, revisit the plan before continuing.

For trivial changes, use an abbreviated workflow. A trivial change is low-risk, localized, and does not meaningfully alter behavior, architecture, security, data flow, persistence, public contracts, or test expectations. State why the abbreviated workflow is appropriate.

## Test Quality Rules

- Tests should precede implementation for non-trivial behavior changes.
- Every changed behavior should have relevant test coverage or a clear explanation of why coverage could not be added.
- New features should include behavioral tests.
- Bug fixes should include regression tests when practical.
- Critical paths should include integration tests where practical.
- Complex logic should include edge-case tests.
- Tests should remain readable, deterministic, isolated, and focused on behavior rather than implementation details.

Avoid these testing anti-patterns:

- Writing tests after implementation for non-trivial behavior changes.
- Snapshot abuse.
- Excessive mocking.
- Brittle implementation-coupled tests.
- Non-deterministic tests.
- Hidden test dependencies.
- Giant monolithic test files.
- Weak assertions that do not prove behavior.
- Deleting or weakening tests to make implementation pass.

## Rust Guidance


## Code Quality Standards

Code must be clear, idiomatic, and consistent with the surrounding codebase.

Prefer:

- Small focused functions.
- Clear module boundaries.
- Meaningful names.
- Explicit data flow.
- Direct control flow.
- Composition over inheritance.
- Localized changes.
- Self-documenting code.
- Minimal useful comments for non-obvious intent.

Avoid:

- Large unreviewed code dumps.
- Excessive abstraction.
- Tight coupling.
- Hidden side effects.
- Premature optimization.
- Massive refactors without scope validation.
- Deeply nested conditionals.
- Giant orchestration methods.
- Dead code.
- Duplicate logic.
- Temporary scaffolding.
- Debug statements.
- Placeholder implementations.

## Cyclomatic Complexity

Keep cyclomatic complexity low.

Guidelines:

- Prefer small focused functions.
- Minimize nested conditionals.
- Prefer composition over branching.
- Extract complex logic into isolated units.
- Avoid giant orchestration methods.

Soft thresholds:

- Recommended: function complexity below 10.
- Strong warning: function complexity above 15.
- Refactor required: function complexity above 20.

Complexity exceptions must be justified.

## Architecture Enforcement

Prefer modular architectures with clear ownership boundaries.

Preferred practices:

- Separation of concerns.
- Explicit interfaces or contracts where useful.
- Dependency injection where it improves testability or extensibility.
- Composition over inheritance.
- Clear layer boundaries.
- Stateless services where practical.
- Externalized configuration.
- Versioning and compatibility awareness for public contracts.

Discouraged patterns:

- God objects.
- Blob classes.
- Service locator abuse.
- Hidden shared mutable state.
- Circular dependencies.
- Tight coupling.
- Massive utility classes.
- Monolithic orchestrators.
- Feature leakage across layers.
- Shotgun surgery.
- Deep inheritance chains.

When introducing or changing architecture, explain:

- The boundary being changed.
- Why the change is needed.
- Alternatives considered.
- Risks and tradeoffs.
- Compatibility impact.
- Test strategy.

## Security Requirements

Treat all external input as untrusted.

Required practices:

- Validate at trust boundaries.
- Sanitize, encode, or parameterize data where appropriate.
- Avoid command injection, path traversal, unsafe deserialization, unsafe reflection, and unsafe dynamic execution.
- Never hard-code secrets, credentials, tokens, private keys, or sensitive environment-specific values.
- Do not expose sensitive data in logs, errors, telemetry, tests, fixtures, screenshots, documentation, or API responses.
- Use secure defaults for authentication, authorization, sessions, cryptography, permissions, and logging.
- Prefer established, maintained security libraries over custom security implementations.

When touching authentication, authorization, cryptography, secret handling, tenant boundaries, profile persistence, filesystem access, or data access controls, explicitly call out security risks and verification performed.

## Error Handling and Reliability

Handle expected errors deliberately and close to the appropriate boundary.

Required practices:

- Fail fast for programmer errors.
- Fail safely for runtime and user-facing errors.
- Provide actionable error messages without leaking sensitive implementation details.
- Avoid swallowed exceptions and silent failures.
- Release resources correctly.
- Consider cancellation, retries, timeouts, idempotency, concurrency, cleanup, and partial failure.
- Make state changes, I/O, network calls, and persistence behavior visible at call sites or documented boundaries.

## Observability and Operations

Add logging, metrics, tracing, or diagnostics where they materially improve operability.

Logs should be:

- Actionable.
- Context-rich.
- Appropriately structured where supported.
- Free of sensitive data.
- Not excessively noisy.

For production-facing changes, consider deployment, rollback, configuration, monitoring, alerting, failure diagnosis, and operational notes where relevant.

## Dependency Management

Minimize new dependencies.

Before introducing a dependency:

- Justify why it is needed.
- Compare against native or existing project solutions.
- Consider maintenance risk.
- Consider security risk.
- Consider license compatibility.
- Consider package size and transitive dependencies.
- Consider whether it duplicates an existing dependency purpose.

External service calls should be isolated behind clear boundaries and should handle network failures, rate limits, retries, timeouts, pagination, schema changes, partial responses, and authentication or authorization failures. Do not assume external systems are reliable, fast, or always available.

## Data and Schema Changes

Treat data migrations, profile persistence changes, world content format changes, and JSON schema changes as production-impacting.

When changing data shape, persistence, or schemas:

- Preserve existing data where possible.
- Make migrations reversible or safely forward-fixable when practical.
- Consider indexing, constraints, transactionality, locking, and performance where applicable.
- Consider compatibility between old and new application versions.
- Validate assumptions about nullability, uniqueness, ordering, and data volume.
- Include migration tests or validation steps where practical.
- Keep schema documents, sample data, and loader validation consistent.

## Performance

Prefer clear code first, then optimize where there is evidence or obvious risk.

Avoid:

- Unnecessary repeated work.
- Excessive allocations.
- Blocking calls in async or goroutine-sensitive paths.
- Unbounded memory growth.
- Inefficient loops over large data.
- Unbounded goroutine creation or concurrency.
- Cache usage without clear invalidation rules.

Only introduce caching when correctness, invalidation, lifecycle, and memory impact are understood. Do not introduce premature complexity for speculative performance gains.

## Documentation Requirements

Update documentation when behavior, setup, public contracts, architecture, operational behavior, or testing expectations change.

Prefer:

- Self-documenting code.
- Clear naming.
- Public API documentation.
- Rationale comments for non-obvious logic.
- Updated setup instructions when commands or environment requirements change.
- Architecture notes when boundaries or dependencies change.

Avoid:

- Redundant comments.
- Commented-out code.
- Narrative noise comments.
- Documentation that describes behavior the code does not enforce.

## Anti-Pattern Detection

Actively detect and flag anti-patterns.

Structural anti-patterns:

- God objects.
- Blob classes.
- Circular dependencies.
- Deep inheritance chains.
- Feature envy.
- Shotgun surgery.
- Monolithic orchestrators.

Maintainability anti-patterns:

- Magic numbers.
- Excessive branching.
- Duplicate logic.
- Dead code.
- Hidden side effects.
- Unclear naming.
- Overly clever abstractions.
- Large files with mixed responsibilities.

Testing anti-patterns:

- Missing tests.
- Mock-heavy brittle tests.
- Giant integration-only coverage.
- Flaky tests.
- Untestable designs.
- Weak assertions.

When an anti-pattern is detected:

- Explain the concern.
- Explain long-term risks.
- Suggest safer alternatives.
- Prefer incremental refactors.

## CI/CD and Review Expectations

When working on build, release, repository health, or pull request preparation, consider:

- Automated test execution.
- Linting.
- Formatting validation.
- Type validation where applicable.
- Dependency scanning where available.
- Security scanning where available.
- Build verification.
- Coverage reporting where available.

Do not claim completion if relevant validation is failing. Large unreviewed changes should be discouraged. Prefer small, reviewable changes with clear summaries.

## Agent Behavior

- Be precise and honest.
- Do not invent files, APIs, commands, libraries, behavior, or test results.
- Do not claim a change was tested unless it was actually tested.
- If verification cannot be completed, explain what was not verified and why.
- When presenting changes, summarize the intent, important design decisions, tests performed, and notable risks.
- Prefer small, cohesive changes over broad rewrites.
- If a requested approach is risky, insecure, brittle, or unmaintainable, explain the concern and choose a safer alternative when possible.

## Definition of Done

A change should be considered complete when feasible only after it:

- Satisfies the requested behavior.
- Fits the existing architecture and conventions.
- Is readable, maintainable, and appropriately extensible.
- Handles relevant errors and edge cases.
- Does not introduce obvious security weaknesses.
- Includes or updates meaningful tests when the change affects behavior, contracts, security, or production risk.
- Runs relevant verification steps where available, or clearly states what could not be verified.
- Avoids unnecessary dependencies, abstractions, and complexity.
- Leaves the codebase cleaner or at least no worse than before.