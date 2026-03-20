# transaction-processing

**Hot-path module** for the Spire modern platform. Synchronous, low-latency (<10ms internal) transaction processing. Gateway, Transactions, Risk & Fraud, and HSM & Crypto communicate in-process.

## Sub-domains

| Sub-domain | Description |
|---|---|
| **Gateway** | TCP listener, ISO 8583 parse, protocol normalization, idempotency, request shaping. |
| **Transactions** | Auth workflow orchestration: validation, enrichment, risk, routing, response. |
| **Risk & Fraud** | Real-time risk evaluation, rules, velocity checks (in-process). |
| **HSM & Crypto** | Hardware-backed crypto: PIN verification, MACing, tokenization, signing (in-process). |

## Tech stack

- **Language:** Go
- **Build:** `go.mod` (single module)
- **Database migrations:** golang-migrate (`migrations/`)
- **Container:** Docker (multi-stage build)
- **CI/CD:** GitHub Actions (`.github/workflows/`)

## Prerequisites

Install these tools before following the Golden Path:

| Tool | Purpose |
|------|---------|
| **Go 1.26+** | Build, run, and test the service. See [Go toolchain](#go-toolchain) for setup. |
| **Git** | Clone the repository. |
| **Docker** | Run the multi-stage build and local containerized services. |
| **Docker Compose** | Start the local database and GCP emulators (Pub/Sub, Spanner) via `docker-compose.yml`. |

Optional (for running DB migrations manually):

| Tool | Purpose |
|------|---------|
| **golang-migrate** | Apply migrations in `migrations/` (e.g. `migrate -path migrations -database "..." up`). |

## Go toolchain

- **Required:** Go 1.26+ (see `go.mod`).
- Install dependencies:

```bash
go mod download
```

## Project structure

```
transaction-processing/
├── .github/workflows/            # CI: build, test, Docker push
├── cmd/                          # Application entry points
│   ├── server/                   # Main application bundle (default entry point)
│   │   └── main.go               # Composes or delegates to modules; single deployable
│   ├── iso-gateway/              # ISO Gateway module entry point
│   │   └── main.go
│   ├── payment/                  # Payment module entry point
│   │   └── main.go
│   ├── risk-fraud/               # Risk & Fraud module entry point
│   │   └── main.go
│   └── hsm-crypto/               # HSM & Crypto module entry point
│       └── main.go
├── internal/                     # Private application packages (not importable)
│   ├── gateway/                  # TCP listener, ISO 8583, request shaping
│   ├── transactions/             # authorization orchestration
│   ├── fraud/                    # risk & fraud evaluation
│   ├── crypto/                   # HSM, PIN, MAC, tokenization
│   ├── iso/                      # ISO 8583 message types and parsing
│   ├── config/                   # app configuration
│   └── middleware/               # Logging, auth, recovery
├── api/                          # OpenAPI specs, proto definitions
├── migrations/                   # DB migrations (golang-migrate)
├── pkg/                          # Shared, importable packages
│   ├── crypto/                   # AES-256-CBC, RSA-OAEP, hash, PKI
│   └── secretvault/              # Secret resolution (e.g. GCP Secret Manager)
├── go.mod                        # Go module definition
├── go.sum                        # Module checksums
├── Dockerfile                    # Container build
└── .gitignore                    # Git ignore rules
```

## Getting started

```bash
# Clone
git clone git@github.com:PayWithSpireInc/transaction-processing.git
cd transaction-processing

# Build
go build ./cmd/server/...

# Run
go run ./cmd/server/

# Run tests (with coverage)
go test ./... -cover

```

## Environment & GCP emulators

Local dependencies (database and GCP emulators for Pub/Sub and Spanner) are defined in `docker-compose.yml`. Use it to run everything required for transaction processing locally:

```bash
docker-compose up -d
```

This brings up the local database and GCP emulators (Pub/Sub, Spanner) so the service can run and be tested without hitting production GCP.

## Golden path setup

Follow these steps for a clean first-time run:

1. **Clone** the repo and enter the directory:
   ```bash
   git clone git@github.com:PayWithSpireInc/transaction-processing.git
   cd transaction-processing
   ```
2. **Start local dependencies:** `docker-compose up -d` (see [Environment & GCP emulators](#environment--gcp-emulators)).
3. **Run the application:** `go run ./cmd/server/` (main entry point).
4. **Run unit tests:** `go test ./...` (with optional `-cover` for coverage).

## Integration testing

Tests that require the database or emulators are gated by the `integration` build tag. Run them only when local dependencies are up:

```bash
go test -tags=integration ./...
```

Unit-only runs (no DB) remain:

```bash
go test ./...
```

## Observability

When the server is running locally, use these endpoints:

| Endpoint        | Purpose                |
|----------------|------------------------|
| `http://localhost:8080/healthz` | Health check (liveness/readiness) |
| `http://localhost:8080/debug/pprof/` | pprof index (if enabled) |
| `http://localhost:8080/metrics` | Prometheus-style metrics (if enabled) |

Adjust the port if your config uses a different value.

## Legacy conflict note

If **BuyPassListener** (the legacy .NET service) is running on the same machine, it may bind to the same port (e.g. **8080**). Stop BuyPassListener or change the Go service port to avoid conflicts.

## Testability

A new team member must be able to **run all tests within 30 minutes** from clone. The Golden Path plus `go test ./...` and `go test -tags=integration ./...` (with `docker-compose up -d`) is designed to meet this "Clone to Test" goal.
