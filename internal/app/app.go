// Package app provides the shared application bootstrap: a small dependency
// container that loads configuration and constructs the structured logger.
// Every Lambda entry point calls app.New at start-up and then injects the
// resulting Config and Logger into its handlers, keeping construction in one
// place and handlers free of global state.
package app

import (
	"io"
	"log/slog"
	"os"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/config"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/logging"
)

// App is the bootstrapped dependency container shared by a Lambda's handlers.
type App struct {
	Config config.Config
	Logger *slog.Logger
}

// New bootstraps the application from the process environment, writing logs to
// stderr. It is the entry point used by Lambda main functions.
func New() (*App, error) {
	return newFrom(os.Getenv, os.Stderr)
}

// newFrom bootstraps from an injected environment lookup and log sink, so the
// bootstrap path is unit-testable without touching process state.
func newFrom(getenv config.Getenv, logSink io.Writer) (*App, error) {
	cfg, err := config.Load(getenv)
	if err != nil {
		return nil, err
	}
	return &App{
		Config: cfg,
		Logger: logging.New(cfg.LogLevel, logSink),
	}, nil
}
