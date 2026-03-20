package appbase

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/samber/do"
	"github.com/samber/lo"
)

// AppBase is the central application container wiring config, logging, and DI.
type AppBase struct {
	Config      *Config
	ServiceName string
	Injector    *do.Injector
}

// Option mutates an AppBase during construction.
type Option func(*AppBase)

// Init loads configuration and bootstraps the zerolog logger.
// lo.Must panics immediately if configuration cannot be read, so a misconfigured
// container never reaches the serving state.
func Init(serviceName string) Option {
	return func(a *AppBase) {
		a.ServiceName = serviceName
		a.Config = lo.Must(LoadConfig())

		level, err := zerolog.ParseLevel(strings.ToLower(a.Config.LogLevel))
		if err != nil {
			level = zerolog.InfoLevel
		}

		var logger zerolog.Logger
		if strings.EqualFold(a.Config.Env, "production") || strings.EqualFold(a.Config.Env, "prod") {
			logger = zerolog.New(os.Stdout).
				With().
				Timestamp().
				Str("service", serviceName).
				Logger()
		} else {
			logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
				With().
				Timestamp().
				Str("service", serviceName).
				Logger()
		}

		log.Logger = logger.Level(level)
		zerolog.SetGlobalLevel(level)
	}
}

// WithDependencyInjector registers all providers in a fresh do.Injector.
// Must be applied after Init so that Config is already populated.
func WithDependencyInjector() Option {
	return func(a *AppBase) {
		a.Injector = NewInjector(a.ServiceName, a.Config)
	}
}

// New constructs an AppBase by applying each Option in order.
func New(opts ...Option) *AppBase {
	app := &AppBase{}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// Shutdown gracefully stops all registered services in the injector.
func (a *AppBase) Shutdown() {
	if a.Injector == nil {
		return
	}
	if err := a.Injector.Shutdown(); err != nil {
		log.Error().Err(err).Msg("appbase: injector shutdown error")
	}
}
