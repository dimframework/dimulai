package main

import (
	"log/slog"
	"os"

	"github.com/dimframework/dim"
	"github.com/dimframework/dimulai"

	_ "github.com/dimframework/dimulai/migrations"
)

func main() {
	// 1. Initialize Logger
	logger := dim.NewLogger(slog.LevelDebug)
	slog.SetDefault(logger.Logger)

	// 2. Load .env file
	if err := dim.LoadEnvFile(".env"); err != nil {
		logger.Debug("No .env file loaded", "error", err)
	}

	// 3. Load Configuration
	cfg, err := dim.LoadConfig()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 4. Initialize Database
	db, err := dim.NewPostgresDatabase(cfg.Database)
	if err != nil {
		logger.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 5. Initialize HTTP Handlers and Routers
	router := dim.NewRouter()
	handler := dimulai.NewAppHandler(db, cfg, router, logger)
	err = handler.LoadRouters()
	if err != nil {
		logger.Error("Failed to load routers", "error", err)
		os.Exit(1)
	}

	// 7. Initialize and Run Console Commands
	console := dim.NewConsole(db, router, cfg)
	console.RegisterBuiltInCommands()

	// Use dedicated migration connection if configured; falls back to write connection.
	if migrationDB, err := dim.NewMigrationDatabase(cfg.Database); err == nil {
		console.WithMigrationDB(migrationDB)
		defer migrationDB.Close()
	}

	if err := console.Run(os.Args[1:]); err != nil {
		logger.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}
