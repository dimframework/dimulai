package dimulai

import (
	"context"
	"testing"
)

func TestMockUserStore(t *testing.T) {
	ctx := context.Background()

	t.Run("Create and FindByID", func(t *testing.T) {
		store := NewMockUserStore()
		user := &User{Name: "Alice", Email: "alice@example.com", Password: "hash"}

		if err := store.Create(ctx, user); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if user.ID == "" {
			t.Fatal("ID should be set after Create")
		}

		found, err := store.FindByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}
		if found.Email != "alice@example.com" {
			t.Errorf("Expected alice@example.com, got %s", found.Email)
		}
	})

	t.Run("FindByID not found", func(t *testing.T) {
		store := NewMockUserStore()
		_, err := store.FindByID(ctx, "nonexistent")
		if err == nil {
			t.Error("Expected error for missing user")
		}
	})

	t.Run("FindByEmail", func(t *testing.T) {
		store := NewMockUserStore()
		user := &User{Name: "Bob", Email: "bob@example.com", Password: "hash"}
		store.Create(ctx, user)

		found, err := store.FindByEmail(ctx, "bob@example.com")
		if err != nil {
			t.Fatalf("FindByEmail failed: %v", err)
		}
		if found.Name != "Bob" {
			t.Errorf("Expected Bob, got %s", found.Name)
		}
	})

	t.Run("FindByEmail not found", func(t *testing.T) {
		store := NewMockUserStore()
		_, err := store.FindByEmail(ctx, "nobody@example.com")
		if err == nil {
			t.Error("Expected error for missing user")
		}
	})

	t.Run("Update", func(t *testing.T) {
		store := NewMockUserStore()
		user := &User{Name: "Carol", Email: "carol@example.com", Password: "hash"}
		store.Create(ctx, user)

		user.Name = "Carol Updated"
		if err := store.Update(ctx, user); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		found, _ := store.FindByID(ctx, user.ID)
		if found.Name != "Carol Updated" {
			t.Errorf("Expected Carol Updated, got %s", found.Name)
		}
	})

	t.Run("Update not found", func(t *testing.T) {
		store := NewMockUserStore()
		err := store.Update(ctx, &User{ID: "ghost", Name: "Ghost"})
		if err == nil {
			t.Error("Expected error updating nonexistent user")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store := NewMockUserStore()
		user := &User{Name: "Dave", Email: "dave@example.com", Password: "hash"}
		store.Create(ctx, user)

		if err := store.Delete(ctx, user.ID); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err := store.FindByID(ctx, user.ID)
		if err == nil {
			t.Error("Expected user to be deleted")
		}
	})

	t.Run("Exists true", func(t *testing.T) {
		store := NewMockUserStore()
		user := &User{Name: "Eve", Email: "eve@example.com", Password: "hash"}
		store.Create(ctx, user)

		exists, err := store.Exists(ctx, "eve@example.com")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("Expected user to exist")
		}
	})

	t.Run("Exists false", func(t *testing.T) {
		store := NewMockUserStore()
		exists, err := store.Exists(ctx, "nobody@example.com")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Expected user to not exist")
		}
	})

	t.Run("UpdatePartial email and name", func(t *testing.T) {
		store := NewMockUserStore()
		user := &User{Name: "Frank", Email: "frank@example.com", Password: "hash"}
		store.Create(ctx, user)

		req := &UpdateUserRequest{}
		req.Email.Present = true
		req.Email.Valid = true
		req.Email.Value = "frank-new@example.com"
		req.Name.Present = true
		req.Name.Valid = true
		req.Name.Value = "Frank New"

		if err := store.UpdatePartial(ctx, user.ID, req); err != nil {
			t.Fatalf("UpdatePartial failed: %v", err)
		}

		found, _ := store.FindByID(ctx, user.ID)
		if found.Email != "frank-new@example.com" {
			t.Errorf("Expected frank-new@example.com, got %s", found.Email)
		}
		if found.Name != "Frank New" {
			t.Errorf("Expected Frank New, got %s", found.Name)
		}
	})

	t.Run("UpdatePartial not found", func(t *testing.T) {
		store := NewMockUserStore()
		req := &UpdateUserRequest{}
		err := store.UpdatePartial(ctx, "ghost", req)
		if err == nil {
			t.Error("Expected error updating nonexistent user")
		}
	})
}

func TestDatabaseUserStore_Delete(t *testing.T) {
	testDB := SetupIntegrationTest(t)
	defer testDB.Cleanup()

	store := NewDatabaseUserStore(testDB.DB)
	ctx := context.Background()

	user := SeedUser(t, testDB.DB, "Delete User", "delete@example.com", "password123")

	if err := store.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.FindByID(ctx, user.ID)
	if err == nil {
		t.Error("Expected user to be deleted from database")
	}
}
