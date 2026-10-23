package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"urlshortener/router"
	"urlshortener/src/config"
	"urlshortener/src/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// the log format depends on the config, so report config problems as plain text
		fmt.Fprintf(os.Stderr, "Invalid configuration:\n%v\n", err)
		os.Exit(1)
	}

	gin.SetMode(cfg.Server.Mode)
	logger := logging.New(cfg.Server.Mode, os.Stdout)
	slog.SetDefault(logger)

	if err := run(cfg, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run connects to the database and serves HTTP until the server fails. It is
// separate from main so deferred cleanup runs before the process exits.
func run(cfg config.Config, logger *slog.Logger) error {
	db, err := sql.Open("postgres", cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}

	r := router.InitRouter(db, cfg.Server.BaseURL, logger)
	logger.Info("listening", "addr", cfg.Server.Addr(), "base_url", cfg.Server.BaseURL, "mode", cfg.Server.Mode)
	return r.Run(cfg.Server.Addr())
}
