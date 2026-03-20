## 1. Goals and Scope

This standard defines how Go services and shared libraries in the modernization program are written, tested, and checked. It covers:

- Test-driven development (TDD)
- Code coverage expectations and enforcement
- Coding style and idioms
- Tooling (formatters, linters, CI integration)

All new Go code must comply. Existing code is brought into compliance incrementally when touched.

Key external references:

- Effective Go
- Google Go Style Guide
- Go coverage docs
- golangci-lint config and linter set

---

## 2. Test-Driven Development (TDD)

### 2.1 Principles

- Prefer **test-first** for all new behavior.
- Default workflow: red → green → refactor:

  - Write a failing test that expresses the new behavior.
  - Implement the minimal code to make it pass.
  - Refactor while keeping tests green.

TDD is mandatory for:

- New services and packages.
- New public APIs in shared libraries.
- Bug fixes where regression risk is non-trivial.

TDD is strongly recommended (but not strictly mandatory) for:

- Internal refactors without behavior change.
- Spike/prototype code that is explicitly marked and not promoted to production.

### 2.2 Test structure and conventions

- Use Go’s standard `testing` package for all tests.
- Use `github.com/stretchr/testify` (or equivalent) where assertions increase clarity, but keep coupling low.
- Test file naming:

  - Unit tests: `foo_test.go` next to `foo.go`.
  - Package-level black-box tests: keep in same package or `<pkg>_test` external package when appropriate.

Test naming:

- Function-level tests: `TestFunctionName_Scenario`.
- Table-driven tests for branchy logic:

```
func TestValidateTransaction(t *testing.T) {
    tests := []struct {
        name    string
        input   Transaction
        wantErr bool
    }{
        {name: "valid transaction", input: validTxn(), wantErr: false},
        {name: "missing account", input: txnMissingAccount(), wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateTransaction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("ValidateTransaction() error = %v, wantErr = %v", err, tt.wantErr)
            }
        })
    }
}

```

This pattern is consistent with Effective Go testing idioms.

### 2.3 What to test

- Core business rules (validation, state transitions, calculations).
- Boundary conditions (limits, empty inputs, error paths).
- Error handling and recovery logic.
- Public APIs of packages (stable contracts).
- For infrastructure adapters (DB, message bus, HTTP clients), prefer:

  - Narrow interfaces
  - Unit tests via mocks/stubs
  - Separate integration tests for real dependencies

**Standardised Tooling for Mocks and Integration Testing [UPDATED] **

~~~panel type=info
**Decision: **The following libraries are standardised across all services. Use these unless a compelling exception is documented and approved by the tech lead.
~~~

- **mockery** ([http://github.com/vektra/mockery](http://github.com/vektra/mockery) ) — **MANDATORY** for all interface mocks.

  - Automatically generates mocks from interfaces; integrates with testify.
  - gomock is not preferred; it requires additional boilerplate and a separate toolchain.
  - Run go generate ./... to regenerate mocks; generated files must be committed.
- **go-wiremock** ([http://github.com/wiremock/go-wiremock](http://github.com/wiremock/go-wiremock) ) — **RECOMMENDED** for integration tests against third-party APIs.

  - Stubs external HTTP APIs so integration tests do not rely on real network calls.
  - Must be confined to ./test or *_integration_test.go files with the integration build tag (see §2.4). Never used in unit test files.
- **apitest** ([http://github.com/steinfletcher/apitest](http://github.com/steinfletcher/apitest) ) — **OPTIONAL** for testing your own HTTP endpoints.

  - Provides a fluent, declarative API over net/http/httptest. Improves readability; not mandated.
  - net/http/httptest remains fully acceptable. Teams may adopt apitest at their discretion.

Do not test:

- Generated code (unless generator is under our control and tests are cheap).
- Trivial getters/setters or pure data structs with no logic.

### 2.4 Integration and E2E tests

- Integration tests live in `./test` or dedicated `*_integration_test.go` with build tags.
- Use Go 1.20+ coverage for integration binaries where practical.
- E2E tests are covered in system-level test plans (see Migration Validation Test Suites) and are not a substitute for unit/integration tests.

---

## 3. Code Coverage

### 3.1 Targets

Minimum thresholds (per service):

- Overall line/statement coverage: **≥ 80%**.
- New or modified packages in a PR: **≥ 90%** coverage unless explicitly waived by tech lead.

These thresholds are enforced in CI via quality gates defined in MOD-23, MOD-51, MOD-53.

Coverage below thresholds:

- Blocks merge unless:

  - Tech lead approves an exception (documented in PR).
  - Work is explicitly tagged as spike/prototype.

### 3.2 How coverage is measured

Standard unit test coverage:

- Command:

```
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out

```

- `go test -coverprofile` and `go tool cover` are the canonical mechanisms.

Integration coverage (Go 1.20+):

- Use coverage-aware builds and `GOCOVERDIR`:

```
# Build and run integration tests with coverage
go test ./... -c -cover

GOCOVERDIR=covdata ./integration-tests-binary

# Aggregate and inspect
go tool covdata textfmt -i=covdata -o=cov.txt
go tool cover -func=cov.txt

```

This follows the Go 1.20 integration coverage guidance.

### 3.3 Where coverage is enforced

- Local: Developers must run `go test ./...` and check coverage before pushing for non-trivial changes.
- CI:

  - MOD-23 (Go build and test pipeline) runs `go test ./... -coverprofile` for all Go modules.
  - MOD-53 (code quality platform) ingests coverage reports and enforces thresholds as a quality gate.
  - MOD-51 (type checking and linting) runs alongside but focuses on lint/type errors, not coverage.

PR checks:

- Failing coverage job blocks merge.
- Coverage report is visible in the PR (via CI UI or code quality platform linked from MOD-53).

### 3.4 Interpreting and improving coverage

Interpretation:

- `go tool cover -func=coverage.out` shows per-function percentages.
- Focus on:

  - Low-coverage functions in critical packages.
  - Error paths and boundary logic not exercised by tests.

Improvement tactics:

- Add tests for:

  - Uncovered branches (`if`, `switch`), especially error branches.
  - Edge cases (empty lists, nil pointers, max/min numeric values).
- Factor complex functions:

  - Extract pure functions for core logic and test them directly.
- Avoid writing tests solely for coverage if they do not encode real behavior expectations.

---

## 4. Coding Style and Standards

Baseline style: Effective Go + Google Go Style Guide.

### 4.1 Formatting

- `gofmt` is mandatory; no manual formatting.
- `goimports` is mandatory for import organization:

  - Standard library
  - Third-party
  - Internal modules
- CI enforces formatting (formatter check in MOD-23).

Rules:

- No unformatted code may be merged.
- IDEs must be configured to format on save using `gofmt`/`goimports`.

### 4.2 Naming conventions

- Packages:

  - Short, lower-case, no underscores: `payments`, `risk`, `settlement`.
  - Avoid stutter: package `payments` should not export `PaymentsService`; prefer `Service`.
- Types and functions:

  - Use PascalCase for exported, camelCase for unexported.
  - Names must be descriptive but concise.
- Interfaces:

  - Prefer behavior-based names: `Store`, `Publisher`, `Hasher`.
  - Avoid `I` prefixes.
- Errors:

  - Use `ErrXxx` for sentinel errors.
  - Error variables should be package-level and unexported unless part of the public contract.

### 4.3 Idiomatic Go patterns

Follow idioms from Effective Go.

Key rules:

- Keep error handling on the happy-path downward:

```
f, err := os.Open(name)
if err != nil {
    return fmt.Errorf("open %s: %w", name, err)
}
defer f.Close()

```

- Prefer simple control flow; avoid deep nesting.
- Use `range` for loops over slices, maps, channels.
- Avoid unnecessary abstractions:

  - Start with simple functions and structs.
  - Introduce interfaces only where multiple implementations are required (e.g., real vs mock).

Context usage [Updated]:

- Always pass `context.Context` as the first parameter for operations that can block or be canceled.
- Do not store `Context` in struct fields.
- Do not create custom `Context` types; use `context.Context`.
- Do not call** **`context.Background()`** **or** **`context.TODO()`** **in a handler, service layer or repository.  
  These calls break distributed tracing, cancellation propagation, and deadline enforcement. Permitted locations only: main() and top-level worker startup, test setup (TestMain or test helpers). All other uses must pass the incoming ctx from the caller.

### 4.4 Error handling

- Functions return `error` as the last return value.
- Wrap errors with `%w` when adding context (see below):

```
if err != nil {
    return fmt.Errorf("load consumer %s: %w", id, err)
}
```

- Avoid logging and returning the same error from deep layers; log once at the boundary (e.g., HTTP handler or top-level worker).
- Use sentinel errors or typed errors only when callers need to branch on specific conditions.

### 4.5 Documentation (godoc)

- All exported types, functions, methods, and packages must have godoc comments.
- Comment format:

  - Start with the name being documented.

```
// ProcessTransaction validates and submits a transaction to the processor.
func ProcessTransaction(ctx context.Context, tx Transaction) error {
    ...
}

```

- Package comments in `doc.go` describe purpose and key concepts.

### 4.6 Concurrency

- Prefer channels and goroutines only when necessary; avoid overusing them.
- Protect shared state with `sync.Mutex` or other primitives.
- Avoid global mutable state; favor dependency injection through constructors.
- Ensure goroutines are tied to `context.Context` and shut down cleanly.
- Every goroutine launch must be accompanied by a documented exit condition — either context cancellation, a done channel, or a bounded lifetime.

~~~panel type=note
**Hard Rule**: Every goroutine launch must have a documented and verifiable exit condition. This is a mandatory code review checkpoint — PRs that launch goroutines without a clear exit strategy must not be merged.
~~~

Required patterns:

- Context cancellation: goroutine selects on ctx.Done() and exits cleanly.
- Done channel: bounded lifetime signalled by a close(done) or similar.
- defer cancel() must be present on every context.WithCancel or context.WithTimeout call — no exceptions.

```go
ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
defer cancel()  // REQUIRED — prevents context leak
go func() {
    select {
    case <-ctx.Done():
        return
    case work := <-queue:
        process(work)
    }
}()
```

Leak detection in tests:** **All services must use [http://go.uber.org/goleak](http://go.uber.org/goleak)  in test setup to detect goroutine leaks automatically:

```
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

Explicit Deadlines for All Blocking Operations [NEW]:

~~~panel type=note
**Hard Rule: **Every outbound call that can block — HTTP, database, message bus, cache — must have an explicit deadline or timeout. Relying on the ambient context timeout alone is not sufficient where the operation has its own tuneable limit.
~~~

Specific requirements:

- Database and message bus clients must be initialised with timeouts sourced from service configuration, not hardcoded literals.
- context.WithTimeout / context.WithDeadline values must be read from configuration (e.g., cfg.GatewayTimeoutSeconds). Hardcoded duration literals require a comment justifying the specific value.
- Wrap all blocking calls inside a context with a deadline; propagate ctx — never create a fresh context.Background() — see §4.3.

```
// WRONG
resp, err := http.DefaultClient.Do(req)

// CORRECT
client := &http.Client{Timeout: cfg.HTTPClientTimeout}
resp, err := client.Do(req)
```

---

## 5. Linters and Static Analysis

Tooling is standardized on `golangci-lint`.

### 5.1 Baseline linter set

The `.golangci.yml` in the repo defines the exact rule set; this section describes the intent.

Core linters (non-exhaustive, subject to versioning in `.golangci.yml`):

- `govet` – language misuse, common bugs.
- `staticcheck` – bug patterns, code smells.
- `gocyclo` / `gocognit` – complexity thresholds.
- `ineffassign`, `unused` – dead code, unused vars.
- `errcheck` – unchecked errors.
- `gosec` – basic security issues.
- Style/consistency linters from go-critic and others (configured in `.golangci.yml`).

Complexity settings:

- `gocognit.min-complexity` set between 10–15 to keep functions small.

### 5.2 Lint rules and decisions

- Lint is treated as non-negotiable; lint errors fail CI.
- Rule suppressions:

  - Prefer local `//nolint:<linter>` comments with justification.
  - Global disables in `.golangci.yml` require tech lead approval and documentation.

Example suppression:

```
// nolint:gosec // MD5 is required here for legacy hashing compatibility with external system.
h := md5.New()

```

### 5.3 Running linters

- Local: `golangci-lint run ./...` before commit.
- CI: MOD-51 runs `golangci-lint` on every PR, and MOD-53 surfaces issues in the code quality dashboard.

The `.golangci.yml` file is the single source of truth for linter configuration and is linked from this document.

---

## 6. Project Structure and Layout

Recommended module layout (per service):

- `cmd/<service-name>/main.go` – service entrypoints.
- `internal/...` – internal packages not for external use.
- `pkg/...` – shared packages intended for reuse.
- `test/...` – cross-package integration or E2E helpers.
- `docs/...` – documentation including this coding standard.

Keep each package focused on a single responsibility:

- `domain` – core domain models and business logic.
- `app` or `usecase` – application services orchestrating domain logic.
- `infra` – external systems (DB, message bus, HTTP clients).

Aligns with clean architecture recommendations and improves testability.

---

## 7. CI, Tooling, and Enforcement

This document is aligned with:

- MOD-23: Go build and test pipeline (build, `go test`, coverage collection).
- MOD-51: Type checking and linting (golangci-lint, static analysis).
- MOD-53: Code quality platform setup (coverage, lint metrics, quality gates).

Enforcement points:

- Pre-merge:

  - Build + tests + coverage threshold.
  - Lint must pass.
  - Formatting check must pass.
- Periodic:

  - Code quality platform reports services with degrading coverage or increasing lint issues.
  - Tech lead reviews metrics and drives remediation for high-risk areas.

Links to the pipeline and code quality dashboards must be added to the Confluence page once MOD-23, MOD-51, MOD-53 are implemented.

---

## 8. Ownership, Versioning, and Changes

Ownership:

- Go Platform Tech Lead owns this document.
- Changes require:

  - PR in repo (`docs/coding-standards-go.md`) reviewed by at least one senior engineer plus the tech lead.
  - Confluence page updated after merge.

Versioning:

- Use semantic versioning in document header:

  - `vMAJOR.MINOR.PATCH` where:

    - MAJOR: breaking changes to expectations (e.g., higher coverage threshold).
    - MINOR: new recommendations or clarifications.
    - PATCH: typo fixes, small clarifications.

Non-compliance:

- CI gates prevent non-compliant code from merging where automated checks exist.
- Manual review enforces items not yet automated (e.g., documentation quality).
- Repeated non-compliance is handled via normal engineering management channels.

---

## 9. Practical References and Config Pointers

- Effective Go
- Google Go Style Guide
- Go coverage docs:

  - `go test -cover`, `-coverprofile`, `go tool cover`
  - Integration coverage (`go build -cover`, `GOCOVERDIR`)
- golangci-lint:

  - Linters and configuration options
  - Project-specific `.golangci.yml` in repo root (link from this section in Confluence).

Repository config pointers (to be filled in when present):

- `.golangci.yml` – linter configuration.
- `Makefile` or task runner – targets:

  - `make test`
  - `make coverage`
  - `make lint`
- CI config:

  - Pipeline definition for MOD-23 (Go build/test).
  - Pipeline definition for MOD-51 (lint/type-check).
  - Quality gate integration for MOD-53 (coverage and lint thresholds).