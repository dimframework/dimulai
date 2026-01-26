package dimulai

import (
	"log/slog"
	"testing"

	"github.com/dimframework/dim"
)

// SetupApp initializes the application handler and router for testing.
// It sets up the integration test database internally.
func SetupApp(t *testing.T) (*dim.Router, *AppHandler, *TestDB) {
	t.Helper()

	testDB := SetupIntegrationTest(t)
	router := dim.NewRouter()
	logger := dim.NewLogger(slog.LevelDebug) // DEBUG MODE

	handler := NewAppHandler(testDB.DB, testDB.Config, router, logger)
	if err := handler.LoadRouters(); err != nil {
		testDB.Cleanup() // Cleanup DB if app load fails
		t.Fatalf("Failed to load routers: %v", err)
	}

	return router, handler, testDB
}
