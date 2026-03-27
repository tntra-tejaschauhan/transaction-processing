# Implementation Plan — MOD-71

## Goal Description

MOD-71 focuses on expanding the [ServerOptions](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options.go#12-30) configuration previously built in MOD-69 and augmenting the TCP read loop built in MOD-70. Specifically, this ticket enforces an explicit idle timeout (disconnecting clients who open a TCP connection but send no data over a sustained period).

We will rely on the [MaxConnections](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options_test.go#142-152) semaphore already implemented in `server.go` and the base `SetReadDeadline` foundation present in [connection.go](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection.go). Tests will be bolstered to catch idle timeouts and slow-writers natively in both unit cases and E2E scenarios.

## User Review Required

> [!WARNING]
> **Minor Logic Correction Proposed for Task 1**:
> The Jira task instructions dictate:
> *“In [processFrame()](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection.go#96-157): after successfully writing response, call `conn.SetReadDeadline(time.Now().Add(opts.IdleTimeout))` — this acts as the idle reset.”*
> 
> However, because [processFrame](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection.go#96-157) loops over every new message, it intrinsically overrides any previous deadline on its first executable line (`c.conn.SetReadDeadline(time.Now().Add(c.opts.ReadTimeout))`).
> 
> **My Proposal to fix this:** 
> I will modify the *first* deadline (waiting for the network header prefix) to use `opts.IdleTimeout` instead. Since waiting for a new frame header is by definition the "idle" wait, this is structurally robust and prevents the deadlines from overwriting each other. The *second* deadline (waiting to read the message body) will maintain the strict `opts.ReadTimeout`. Please confirm if you approve this logical fix.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/appbase/config.go)
- Add `IdleTimeoutMs int` to [GatewayConfig](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/appbase/config.go#14-21) struct.
- Apply structural tags: `yaml:"idle_timeout_ms" env:"GATEWAY_IDLE_TIMEOUT_MS" env-default:"30000"`.

#### [MODIFY] [gateway.yaml](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/config/gateway.yaml)
- Add `idle_timeout_ms: 30000`.

#### [MODIFY] [options.go](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options.go)
- Add `IdleTimeout time.Duration` to [ServerOptions](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options.go#12-30).
- Include `IdleTimeout` inside [NewServerOptions()](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options.go#31-56)'s allocation map.
- In [validate()](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options.go#57-76), assert `o.IdleTimeout > o.ReadTimeout` strictly per Jira definitions.

---

### Core TCP Server

#### [MODIFY] [connection.go](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection.go)
- Check [err](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options_test.go#78-99) returned by [readLoop](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection.go#63-95) safely inside [handle()](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection.go#40-62). Look for standard timeout signatures (`os.IsTimeout(err)`) and explicitly trigger `c.logger.Info().Msg("connection idle timeout")` for matching cases.
- In [processFrame()](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection.go#96-157): Set the *very first* read deadline (the header prefix read) to `opts.IdleTimeout`.
- Retain the strict `opts.ReadTimeout` for the secondary body read.

---

### Unit & E2E Testing

#### [MODIFY] [options_test.go](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/options_test.go)
- Write tests enforcing `IdleTimeout > ReadTimeout` logic bounds.

#### [MODIFY] [connection_test.go](file:///home/tntra/Desktop/Spire/poc/transaction-processing/internal/gateway/server/connection_test.go)
- Establish an `IdleTimeout` validation test. Specifically: open connection -> do nothing -> assert connection dropped after duration `opts.IdleTimeout + 50ms`.

#### [MODIFY] [echo_e2e_test.go](file:///home/tntra/Desktop/Spire/poc/transaction-processing/api/gateway/echo_e2e_test.go)
- Implement `TestConnectionLimit`: Target the server with `MaxConnections + 1` concurrent TCP dials (`net.Dial`), verify the `n+1` connection rejects actively while original connections pass successfully.
- Implement `TestIdleTimeout`: Hold a raw TCP connection dormant, ensure termination.
- Implement `TestSlowClientWriteTimeout`: Send half an underlying packet structure intentionally stalling on standard `WriteTimeout`, ensuring server executes disconnect.

## Verification Plan
1. Execution of E2E verification test suite ensuring race mitigation and stability against large dormant connection blocks.
2. Standard test coverage evaluation via `go test -race -cover ./internal/gateway/...` maintaining `80%+`.
