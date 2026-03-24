# MOD-69 — Project Structure Tree
# transaction-processing-develop (existing monorepo)
#
# Legend:
#   [NEW]      Created as part of MOD-69
#   [EXISTING] Already in the repo — only minor additions needed
#   ✦          Touch point — add deps / targets / fields, do not rewrite
# ──────────────────────────────────────────────────────────────────────

transaction-processing-develop/
│
├── .github/                                   [EXISTING] ✦ extend CI for gateway
│   └── workflows/
│       └── ci.yml                             [EXISTING] ✦ add test-gateway + build-gateway jobs
│
├── api/
│   └── gateway/                               [NEW] E2E test harness — TASK-7
│       ├── e2e_client.go                      [NEW] ISO TCP test client (dial, frame 0800, read 0810)
│       └── echo_e2e_test.go                   [NEW] //go:build e2e — 3 acceptance scenarios
│
├── cmd/
│   ├── gateway/                               [EXISTING]
│   │   └── main.go                            [EXISTING] ✦ wire server + ISO pipeline — TASK-6
│   └── simulator/                             [NEW] CLI manual-test tool — TASK-8
│       └── main.go                            [NEW] --host --port → send 0800, print 0810
│
├── docs/                                      [EXISTING]
│
├── internal/
│   ├── appbase/                               [EXISTING]
│   │
│   ├── config/                                [EXISTING] ✦ add GatewayConfig struct — TASK-2
│   │
│   ├── crypto/                                [EXISTING]
│   │
│   ├── fraud/                                 [EXISTING]
│   │
│   ├── gateway/                               [NEW] ◄─ ALL MOD-69 code lives here
│   │   │
│   │   ├── iso/                               [NEW] ISO 8583 package — TASK-5
│   │   │   ├── spec.go                        [NEW] DiscoverSpec *iso8583.MessageSpec
│   │   │   │                                        (all Discover fields: F0,F1,F11,F37,
│   │   │   │                                         F39,F41,F42,F70 + full bitmap)
│   │   │   ├── types.go                       [NEW] EchoRequest / EchoResponse structs
│   │   │   │                                        with iso8583 struct tags
│   │   │   ├── handler.go                     [NEW] HandleMessage() — MTI dispatcher
│   │   │   ├── echo.go                        [NEW] BuildEcho0810() — 0800 → 0810
│   │   │   ├── framing.go                     [NEW] NetworkHeader = network.Binary2Bytes
│   │   │   │                                        (moov-io/iso8583-connection)
│   │   │   ├── spec_test.go                   [NEW] Round-trip Pack/Unpack unit test
│   │   │   ├── echo_test.go                   [NEW] 0810 builder assertions
│   │   │   └── iso_bench_test.go              [NEW] BenchmarkPack, BenchmarkUnpack
│   │   │
│   │   └── server/                            [NEW] TCP server package — TASK-3 & TASK-4
│   │       ├── server.go                      [NEW] Server struct, New(), Start(),
│   │       │                                        Stop(), acceptLoop()
│   │       ├── connection.go                  [NEW] Conn, handle(), readLoop(), Write()
│   │       ├── options.go                     [NEW] ServerOptions (port, timeouts,
│   │       │                                        max connections) — TASK-2
│   │       ├── metrics.go                     [NEW] Prometheus gauges:
│   │       │                                        active_connections, accept_errors_total
│   │       ├── server_test.go                 [NEW] Lifecycle unit tests (net.Pipe)
│   │       └── connection_test.go             [NEW] Read/write framing unit tests
│   │
│   ├── iso/                                   [EXISTING] Unrelated — do NOT modify
│   ├── middleware/                             [EXISTING]
│   └── transactions/                          [EXISTING]
│
├── migrations/                                [EXISTING]
├── pkg/                                       [EXISTING]
│
├── go.mod                                     [EXISTING] ✦ go get moov-io deps — TASK-1
├── go.sum                                     [EXISTING] ✦ updated by go get
├── .golangci.yml                              [EXISTING] ✦ verify internal/gateway covered
├── .gitignore                                 [EXISTING]
├── Makefile                                   [EXISTING] ✦ add targets below — TASK-1
├── server                                     [EXISTING] do NOT modify
└── README.md                                  [EXISTING]

# ──────────────────────────────────────────────────────────────────────
# NEW Makefile targets to add (TASK-1)
# ──────────────────────────────────────────────────────────────────────

  build-gateway      go build -o bin/gateway ./cmd/gateway
  build-simulator    go build -o bin/simulator ./cmd/simulator
  test-gateway       go test ./internal/gateway/...
  test-gateway-race  go test -race ./internal/gateway/...
  bench-gateway      go test -bench=. ./internal/gateway/iso/...
  e2e-gateway        (start bin/gateway, wait :8583, run go test -tags e2e ./api/gateway/...)

# ──────────────────────────────────────────────────────────────────────
# NEW go.mod dependencies to add (TASK-1)
# ──────────────────────────────────────────────────────────────────────

  github.com/moov-io/iso8583            ISO 8583 message spec, Marshal/Unmarshal
  github.com/moov-io/iso8583-connection TCP framing — network.Binary2Bytes header
  github.com/prometheus/client_golang   Metrics
  go.uber.org/zap                       Structured logging
  github.com/spf13/viper                Config (YAML + env vars)
  github.com/stretchr/testify           Test assertions

# ──────────────────────────────────────────────────────────────────────
# Key rules
# ──────────────────────────────────────────────────────────────────────

  • All new Go files for MOD-69 go under internal/gateway/ only
  • internal/iso/ (existing) is unrelated — do not touch it
  • Do NOT create a new go.mod — extend the existing monorepo module
  • E2E tests in api/gateway/ use //go:build e2e tag (excluded from go test ./...)
  • The `server` file at repo root is an existing artifact — do not modify or delete it


-------------------------------------------------------
build-gateway:
	go build -o bin/gateway ./cmd/gateway

build-simulator:
	go build -o bin/simulator ./cmd/simulator

run simulator:

./bin/gateway 
./bin/simulator --stan 999999 --code 999

stop app :
pkill -f "bin/gateway" || true