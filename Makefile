.PHONY: build build-gateway build-simulator test test-gateway test-gateway-race bench-gateway e2e-gateway signoff-gateway lint

# ── Build ─────────────────────────────────────────────────────────────────────
build:
	go build ./...

build-gateway:
	go build -o bin/gateway ./cmd/gateway

build-simulator:
	go build -o bin/simulator ./cmd/simulator

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
	@./bin/gateway &
	@sleep 1
	go test -tags e2e -cover ./api/gateway/...
	@pkill -f bin/gateway || true

# ── Sign-off (Discover acceptance criteria, requires running gateway binary) ──
signoff-gateway: build-gateway
	@echo "Starting gateway..."
	@./bin/gateway &
	@sleep 1
	go test -tags e2e -run ^TestSignOff_ -v ./api/gateway/...
	@pkill -f bin/gateway || true

# ── Lint ──────────────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./...
