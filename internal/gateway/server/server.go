package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

// metrics holds Prometheus gauges for the TCP server.
// promauto registers them automatically on package init.
var (
	metricActiveConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_active_connections",
		Help: "Number of currently active TCP connections.",
	})

	metricTotalAccepts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_total_accepts_total",
		Help: "Total number of accepted TCP connections.",
	})

	metricAcceptErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_accept_errors_total",
		Help: "Total number of errors encountered during Accept().",
	})
)

// Server is the ISO 8583 TCP gateway server.
// It accepts persistent TCP connections from the card network,
// dispatches each connection to a goroutine, and shuts down cleanly.
type Server struct {
	opts     *ServerOptions
	logger   zerolog.Logger
	listener net.Listener

	// sem is a counting semaphore that enforces MaxConnections.
	// Acquiring a slot = one active connection; releasing = connection done.
	sem chan struct{}

	wg     sync.WaitGroup
	cancel context.CancelFunc
	mu     sync.Mutex // protects listener during Stop()
}

// New creates a new Server and opens the TCP listener on opts.Port.
// Returns an error if the port cannot be bound.
func New(opts *ServerOptions, logger zerolog.Logger) (*Server, error) {
	addr := fmt.Sprintf(":%d", opts.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("server: listen on %s: %w", addr, err)
	}

	logger.Info().
		Int("port", opts.Port).
		Int("max_connections", opts.MaxConnections).
		Dur("read_timeout", opts.ReadTimeout).
		Dur("shutdown_timeout", opts.ShutdownTimeout).
		Msg("gateway server created")

	return &Server{
		opts:     opts,
		logger:   logger,
		listener: ln,
		sem:      make(chan struct{}, opts.MaxConnections),
	}, nil
}

// Start begins accepting connections.
// It derives a child context from ctx so Stop() can cancel it independently.
// Start is non-blocking — the accept loop runs in a background goroutine.
//
// Goroutine exit condition: acceptLoop exits when ctx is cancelled (via Stop)
// or when the listener is closed — whichever comes first.
func (s *Server) Start(ctx context.Context) error {
	// Derive a child context so Stop() can cancel the loop without
	// affecting the parent context passed in by the caller.
	childCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.wg.Add(1)
	go func() {
		// Exit condition: acceptLoop returns on listener close or ctx cancel.
		defer s.wg.Done()
		s.acceptLoop(childCtx)
	}()

	s.logger.Info().Int("port", s.opts.Port).Msg("gateway server started")
	return nil
}

// Stop gracefully shuts down the server.
// It cancels the accept loop, closes the listener, and waits for all
// active connection goroutines to finish — up to ShutdownTimeout.
func (s *Server) Stop() {
	s.logger.Info().Msg("gateway server stopping")

	// Cancel the accept loop context first.
	if s.cancel != nil {
		s.cancel()
	}

	// Close the listener so any blocked Accept() call unblocks immediately.
	s.mu.Lock()
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.mu.Unlock()

	// Wait for all goroutines to finish, bounded by ShutdownTimeout.
	// Exit condition for the wait: timer fires or all goroutines done.
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info().Msg("gateway server stopped cleanly")
	case <-time.After(s.opts.ShutdownTimeout):
		s.logger.Warn().
			Dur("timeout", s.opts.ShutdownTimeout).
			Msg("gateway server shutdown timed out — some connections may have been force-closed")
	}
}

// acceptLoop is the core accept loop.
// It runs until the listener is closed or ctx is cancelled.
//
// Exit condition: net.Listen close unblocks Accept() with an error,
// which causes the loop to return. ctx.Done() is also checked after
// each temporary error.
func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we are shutting down — a closed listener returns
			// an error that is NOT a timeout, so check ctx first.
			select {
			case <-ctx.Done():
				s.logger.Info().Msg("accept loop: context cancelled, exiting")
				return
			default:
			}

			// Temporary/timeout errors: log, back off briefly, retry.
			var netErr net.Error
			if ok := isNetError(err, &netErr); ok && netErr.Timeout() {
				s.logger.Warn().Err(err).Msg("accept loop: temporary error, retrying")
				metricAcceptErrors.Inc()
				time.Sleep(5 * time.Millisecond)
				continue
			}

			// Any other error is fatal — the listener is broken.
			s.logger.Error().Err(err).Msg("accept loop: fatal error, stopping")
			metricAcceptErrors.Inc()
			return
		}

		metricTotalAccepts.Inc()

		// Try to acquire a semaphore slot (non-blocking).
		// If MaxConnections is reached, reject the connection immediately.
		select {
		case s.sem <- struct{}{}:
			// Slot acquired — proceed.
		default:
			s.logger.Warn().
				Str("remote_addr", conn.RemoteAddr().String()).
				Int("max_connections", s.opts.MaxConnections).
				Msg("accept loop: max connections reached, rejecting connection")
			_ = conn.Close()
			continue
		}

		// Dispatch the connection to its own goroutine.
		// Exit condition: handleConn returns when the connection closes,
		// an error occurs, or ctx is cancelled.
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer func() { <-s.sem }() // release semaphore slot on exit

			metricActiveConns.Inc()
			defer metricActiveConns.Dec()

			s.handleConn(ctx, c)
		}(conn)
	}
}

// handleConn delegates connection lifecycle to Conn.
// Called from acceptLoop — each invocation runs in its own goroutine.
// WaitGroup and semaphore release are managed by the caller in acceptLoop.
func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	c := newConn(conn, s.opts, s.logger)
	c.handle(ctx)
}

// isNetError is a helper that type-asserts err to net.Error.
// Extracted to make acceptLoop unit-testable.
func isNetError(err error, target *net.Error) bool {
	var netErr net.Error
	if ok := asNetError(err, &netErr); ok {
		*target = netErr
		return true
	}
	return false
}

func asNetError(err error, target *net.Error) bool {
	netErr, ok := err.(net.Error) //nolint:errorlint // net.Error is an interface
	if ok {
		*target = netErr
	}
	return ok
}
