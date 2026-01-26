package dimulai

import (
	"testing"

	"github.com/dimframework/dim"
)

// SeedAuth generates a valid access token for a user
func SeedAuth(t *testing.T, user *User, jwtConfig *dim.JWTConfig) string {
	t.Helper()

	jwtManager, err := dim.NewJWTManager(jwtConfig)
	if err != nil {
		t.Fatalf("Failed to init JWT manager: %v", err)
	}

	sessionID := dim.NewUuid().String()
	token, err := jwtManager.GenerateAccessToken(user.GetID(), user.GetEmail(), sessionID, nil)
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}

	return token
}
