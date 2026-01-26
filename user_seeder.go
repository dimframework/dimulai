package dimulai

import (
	"context"
	"testing"

	"github.com/dimframework/dim"
)

// SeedUser creates a test user in the database
func SeedUser(t *testing.T, db dim.Database, name, email, password string) *User {
	t.Helper()

	hashedPassword, err := dim.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &User{
		Name:     name,
		Email:    email,
		Password: hashedPassword,
	}

	store := NewPostgresUserStore(db)
	if err := store.Create(context.Background(), user); err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	return user
}
