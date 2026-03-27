.PHONY: build build-gateway build-simulator test test-gateway test-gateway-race bench-gateway e2e-gateway lint

# ── Build ─────────────────────────────────────────────────────────────────────
build:
	go build ./...

build-gateway:
	go build -o bin/gateway ./cmd/gateway

build-simulator:
	go build -o bin/simulator ./cmd/simulator

# Alias matching the doc requirement: make simulator
simulator: build-simulator

# ── Test ──────────────────────────────────────────────────────────────────────
test:
	go test ./...

test-gateway:
	go test ./internal/gateway/...

test-gateway-race:
	go test -race ./internal/gateway/...

bench-gateway:
	go test -bench=. -benchmem ./internal/gateway/iso/...

# ── E2E (requires running gateway binary) ────────────────────────────────────
e2e-gateway: build-gateway
	@echo "Starting gateway..."
	@GATEWAY_MAX_CONNECTIONS=5 GATEWAY_IDLE_TIMEOUT_MS=2000 GATEWAY_READ_TIMEOUT_MS=1000 ./bin/gateway &
	@sleep 1
	go test -tags e2e -v ./api/gateway/...
	@pkill -f bin/gateway || true

# ── Lint ──────────────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./...
