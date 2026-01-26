package dimulai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dimframework/dim"
)

func TestUserSeeding_Diagnostic(t *testing.T) {
	testDB := SetupIntegrationTest(t)
	defer testDB.Cleanup()

	// Test 1: Create a user
	t.Run("Seed user should persist in database", func(t *testing.T) {
		user := SeedUser(t, testDB.DB, "Test User", "seed@example.com", "password123")

		if user.ID == "" {
			t.Fatal("User ID is empty after seeding")
		}

		t.Logf("Seeded user with ID: %s, Email: %s", user.ID, user.Email)

		// Verify user was persisted
		userStore := NewPostgresUserStore(testDB.DB)
		foundUser, err := userStore.FindByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("Failed to find user after seeding: %v", err)
		}

		if foundUser.GetID() != user.ID {
			t.Errorf("Expected user ID %s, got %s", user.ID, foundUser.GetID())
		}

		if foundUser.GetEmail() != user.Email {
			t.Errorf("Expected user email %s, got %s", user.Email, foundUser.GetEmail())
		}

		t.Log("✓ User seeding verified successfully")
	})

	// Test 2: Generate token and verify it can be used
	t.Run("Generated token should be valid", func(t *testing.T) {
		user := SeedUser(t, testDB.DB, "Token User", "token@example.com", "password123")
		token := SeedAuth(t, user, &testDB.Config.JWT)

		if token == "" {
			t.Fatal("Generated token is empty")
		}

		t.Logf("Generated token: %s...", token[:20])
		t.Log("✓ Token generation verified successfully")
	})
}

func TestUserHandler_Me(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed User
	user := SeedUser(t, testDB.DB, "Me User", "me@example.com", "password123")
	token := SeedAuth(t, user, &testDB.Config.JWT)

	// DEBUG: Verify user exists
	var count int
	err := testDB.DB.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE id = $1", user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}
	if count == 0 {
		t.Fatalf("User %s not found in DB after seed", user.ID)
	}

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp User
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.ID != user.ID {
			t.Errorf("Expected user ID %s, got %s", user.ID, resp.ID)
		}
		if resp.Email != user.Email {
			t.Errorf("Expected user Email %s, got %s", user.Email, resp.Email)
		}
	})

	t.Run("Unauthorized_NoToken", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		// No Authorization header
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Unauthorized_InvalidToken", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 Unauthorized, got %d", rec.Code)
		}
	})
}

func TestUserHandler_ChangePassword(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed User
	password := "OldPassword123!"
	user := SeedUser(t, testDB.DB, "CP User", "cp@example.com", password)
	token := SeedAuth(t, user, &testDB.Config.JWT)

	t.Run("Success", func(t *testing.T) {
		payload := map[string]string{
			"old_password": password,
			"new_password": "NewPassword123!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/password", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
			return
		}

		// Verify password was updated correctly in DB
		var newHash string
		err := testDB.DB.QueryRow(context.Background(), "SELECT password FROM users WHERE id = $1", user.ID).Scan(&newHash)
		if err != nil {
			t.Errorf("Failed to query password from DB: %v", err)
			return
		}
		if err := dim.VerifyPassword(newHash, "NewPassword123!"); err != nil {
			t.Error("Password was not updated correctly in DB")
		}
	})

	t.Run("WrongOldPassword", func(t *testing.T) {
		payload := map[string]string{
			"old_password": "WrongPassword",
			"new_password": "NewPassword123!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/password", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 Unauthorized, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestUserHandler_UpdateProfile(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed User
	user := SeedUser(t, testDB.DB, "Profile User", "profile@example.com", "password")
	token := SeedAuth(t, user, &testDB.Config.JWT)

	// Seed Another User for Conflict Check
	SeedUser(t, testDB.DB, "Other User", "other@example.com", "password")

	t.Run("Success_NameOnly", func(t *testing.T) {
		payload := map[string]string{
			"name": "Updated Name",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/me", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp User
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Name != "Updated Name" {
			t.Errorf("Expected name Updated Name, got %s", resp.Name)
		}
		if resp.Email != "profile@example.com" { // Email should remain unchanged
			t.Errorf("Expected email profile@example.com, got %s", resp.Email)
		}
	})

	t.Run("Conflict_Email", func(t *testing.T) {
		payload := map[string]string{
			"email": "other@example.com",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/me", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("Expected status 409 Conflict, got %d", rec.Code)
		}
	})
}

func TestTokenClaimsVerification(t *testing.T) {
	testDB := SetupIntegrationTest(t)
	defer testDB.Cleanup()

	// Seed user
	user := SeedUser(t, testDB.DB, "Token Test User", "tokentest@example.com", "password123")
	token := SeedAuth(t, user, &testDB.Config.JWT)

	t.Run("Token_claims_contain_correct_user_id", func(t *testing.T) {
		// Verify token is valid and contains correct user ID
		jwtManager, err := dim.NewJWTManager(&testDB.Config.JWT)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		claims, err := jwtManager.VerifyToken(token)
		if err != nil {
			t.Fatalf("Failed to verify token: %v", err)
		}

		tokenUserID, ok := claims["sub"].(string)
		if !ok {
			t.Fatalf("Failed to extract user ID from token claims")
		}

		if tokenUserID != user.ID {
			t.Errorf("Token user ID mismatch. Expected %s, got %s", user.ID, tokenUserID)
		}

		// Verify user can be found in DB with this ID
		userStore := NewPostgresUserStore(testDB.DB)
		foundUser, err := userStore.FindByID(context.Background(), tokenUserID)
		if err != nil {
			t.Errorf("Failed to find user in DB with token user ID: %v", err)
		}

		if foundUser.GetID() != user.ID {
			t.Errorf("DB user ID mismatch. Expected %s, got %s", user.ID, foundUser.GetID())
		}

		t.Log("✓ Token claims and database user ID match correctly")
	})
}

func TestChangePasswordHandlerLogic(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed user in the SAME database as the app
	password := "OldPassword123!"
	user := SeedUser(t, testDB.DB, "CP User", "cp@example.com", password)
	token := SeedAuth(t, user, &testDB.Config.JWT)

	t.Run("Step1_VerifyUserExists", func(t *testing.T) {
		userStore := NewPostgresUserStore(testDB.DB)
		foundUser, err := userStore.FindByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("User not found in DB: %v", err)
		}
		t.Logf("✓ User found in DB: %s (%s)", foundUser.GetID(), foundUser.GetEmail())
	})

	t.Run("Step2_VerifyTokenValid", func(t *testing.T) {
		jwtManager, err := dim.NewJWTManager(&testDB.Config.JWT)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		claims, err := jwtManager.VerifyToken(token)
		if err != nil {
			t.Fatalf("Token verification failed: %v", err)
		}

		tokenUserID, _ := claims["sub"].(string)
		t.Logf("✓ Token valid with user ID: %s", tokenUserID)
	})

	t.Run("Step3_FullChangePasswordRequest", func(t *testing.T) {
		payload := map[string]string{
			"old_password": password,
			"new_password": "NewPassword123!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/password", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Response: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestRouterRegistration(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	t.Run("PUT_me_route_exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me", bytes.NewBuffer([]byte(`{}`)))
		req.Header.Set("Authorization", "Bearer invalid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// We expect 401 (invalid token) or 200 (route exists), NOT 404 (route not found)
		if rec.Code == http.StatusNotFound {
			t.Errorf("Route PUT /api/v1/me not found (404). Body: %s", rec.Body.String())
		}
		t.Logf("PUT /api/v1/me returned status: %d", rec.Code)
	})

	t.Run("PUT_me_password_route_exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/password", bytes.NewBuffer([]byte(`{}`)))
		req.Header.Set("Authorization", "Bearer invalid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// We expect 401 (invalid token) or 200 (route exists), NOT 404 (route not found)
		if rec.Code == http.StatusNotFound {
			t.Errorf("Route PUT /api/v1/me/password not found (404). Body: %s", rec.Body.String())
		}
		t.Logf("PUT /api/v1/me/password returned status: %d", rec.Code)
	})
}
