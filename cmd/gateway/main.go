// Command gateway is the ISO 8583 TCP gateway for the Discover card network.
// See MOD-69: TCP connection acceptance and lifecycle.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/PayWithSpireInc/transaction-processing/internal/appbase"
	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/server"
)

const (
	configPath  = "internal/config/gateway.yaml"
	metricsAddr = ":9090"
)

func main() {
	// Bootstrap zerolog logger and app config via existing appbase pattern.
	// context.Background() is permitted here — this is the top-level entry point.
	app := appbase.New(
		appbase.Init("gateway"),
	)
	defer app.Shutdown()

	// Load gateway-specific config from YAML + env overrides.
	opts, err := server.NewServerOptions(configPath)
	if err != nil {
		log.Fatal().Err(err).Str("config", configPath).Msg("failed to load gateway config")
	}

	// Construct the TCP server — binds the port immediately.
	srv, err := server.New(opts, log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create gateway server")
	}

	// Start the accept loop — non-blocking.
	// context.Background() permitted here — top-level worker startup.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start gateway server")
	}

	log.Info().
		Int("port", opts.Port).
		Str("metrics", metricsAddr).
		Msg("gateway running")

	// Start Prometheus metrics + healthz on a separate port.
	// Exit condition: goroutine runs until process exits.
	go func() {
		mux := http.NewServeMux()

		// Prometheus metrics endpoint.
		mux.Handle("/metrics", promhttp.Handler())

		// Health check endpoint — returns 200 OK while server is running.
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		metricsSrv := &http.Server{
			Addr:    metricsAddr,
			Handler: mux,
		}

		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Str("addr", metricsAddr).Msg("metrics server error")
		}
	}()

	// Block until SIGTERM or SIGINT.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info().Msg("shutdown signal received")

	// Graceful shutdown — drains active connections within ShutdownTimeout.
	srv.Stop()

	log.Info().Msg("gateway exited")
}
