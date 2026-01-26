package dimulai

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/dimframework/dim"
	_ "github.com/dimframework/dimulai/migrations" // Ensure app migrations are registered
)

// TestDB holds the database connection and config for testing
type TestDB struct {
	DB      *dim.PostgresDatabase
	Config  *dim.Config
	cleanup func()
}

// SetupIsolatedSchemaTest prepares a test database with an isolated schema.
// It loads configuration, creates a unique schema, runs migrations,
// and returns the database connection and a cleanup function.
func SetupIsolatedSchemaTest(t *testing.T) *TestDB {
	t.Helper()

	// 1. Load Configuration
	// ONLY load .env.test. NEVER load .env for tests to prevent accidental
	// modification of local dev database.
	if err := dim.LoadEnvFile(".env.test"); err != nil {
		// If .env.test is missing, we don't fallback to .env.
		// We allow proceeding in case env vars are already set via CI environment.
		// Using fmt.Println instead of t.Log for better visibility in some environments.
		fmt.Println("⚠ Warning: .env.test not found. Using system environment variables.")
	}

	cfg, err := dim.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 2. Create Unique Schema Name
	uuid := dim.NewUuid()
	schema := "t_" + strings.ReplaceAll(uuid.String(), "-", "")

	// 3. Create Schema using Base Connection (Default Search Path)
	// We need a temporary connection to create the schema
	baseDB, err := dim.NewPostgresDatabase(cfg.Database)
	if err != nil {
		t.Fatalf("Failed to connect to base database: %v", err)
	}

	if err := baseDB.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)); err != nil {
		baseDB.Close()
		t.Fatalf("Failed to create schema %s: %v", schema, err)
	}
	baseDB.Close() // Close base connection immediately

	// 4. Connect with Scoped Schema
	// Clone config and modify search_path
	scopedCfg := *cfg
	// Copy the map to avoid modifying the original config shared state (shallow copy issue)
	scopedCfg.Database.RuntimeParams = make(map[string]string)
	for k, v := range cfg.Database.RuntimeParams {
		scopedCfg.Database.RuntimeParams[k] = v
	}
	scopedCfg.Database.RuntimeParams["search_path"] = schema

	// Create scoped connection
	scopedDB, err := dim.NewPostgresDatabase(scopedCfg.Database)
	if err != nil {
		// Try to clean up schema if connection fails
		cleanupSchema(cfg, schema)
		t.Fatalf("Failed to connect to test database (schema=%s): %v", schema, err)
	}

	// 5. Run Migrations
	// Get all migrations (Framework + Registered)
	migrations := dim.GetFrameworkMigrations()
	migrations = append(migrations, dim.GetRegisteredMigrations()...)

	if err := dim.RunMigrations(scopedDB, migrations); err != nil {
		scopedDB.Close()
		cleanupSchema(cfg, schema)
		t.Fatalf("Failed to run migrations in isolated schema %s: %v", schema, err)
	}

	// 6. Return TestDB with Cleanup
	return &TestDB{
		DB:     scopedDB,
		Config: &scopedCfg,
		cleanup: func() {
			scopedDB.Close()
			cleanupSchema(cfg, schema)
		},
	}
}

// cleanupSchema removes the isolated schema using a fresh connection
func cleanupSchema(cfg *dim.Config, schema string) {
	dropDB, err := dim.NewPostgresDatabase(cfg.Database)
	if err != nil {
		// Log error but don't fail, we are in cleanup
		fmt.Fprintf(os.Stderr, "Failed to connect for cleanup schema %s: %v\n", schema, err)
		return
	}
	defer dropDB.Close()

	if err := dropDB.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to drop schema %s: %v\n", schema, err)
	}
}

// Cleanup calls the cleanup function
func (tdb *TestDB) Cleanup() {
	if tdb.cleanup != nil {
		tdb.cleanup()
	}
}

// SetupIntegrationTest is a helper for integration tests
func SetupIntegrationTest(t *testing.T) *TestDB {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Silence logger for tests
	originalLogger := slog.Default()
	logger := dim.NewLogger(slog.LevelError)
	slog.SetDefault(logger.Logger)
	defer slog.SetDefault(originalLogger)

	// Actually, we probably want to keep logging but maybe redirect it or filter it.
	// For now, let's just run SetupIsolatedSchemaTest
	return SetupIsolatedSchemaTest(t)
}
