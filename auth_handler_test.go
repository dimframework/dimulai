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

func TestAuthHandler_Register(t *testing.T) {
	// Setup App
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	t.Run("Success", func(t *testing.T) {
		payload := map[string]string{
			"name":     "Test User",
			"email":    "test@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("Expected status 201 Created, got %d", rec.Code)
		}

		var resp User
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Email != payload["email"] {
			t.Errorf("Expected email %s, got %s", payload["email"], resp.Email)
		}
	})

	t.Run("DuplicateEmail", func(t *testing.T) {
		// Seed first user
		SeedUser(t, testDB.DB, "Existing User", "duplicate@example.com", "password123")

		payload := map[string]string{
			"name":     "New User",
			"email":    "duplicate@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("Expected status 409 Conflict, got %d", rec.Code)
		}
	})

	t.Run("InvalidInput", func(t *testing.T) {
		payload := map[string]string{
			"name":     "", // Invalid: required
			"email":    "invalid-email",
			"password": "123", // Invalid: too short
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 Bad Request, got %d", rec.Code)
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	// Setup App
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed User
	email := "login@example.com"
	password := "password123"
	SeedUser(t, testDB.DB, "Login User", email, password)

	t.Run("Success", func(t *testing.T) {
		payload := map[string]string{
			"email":    email,
			"password": password,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK, got %d", rec.Code)
		}

		var resp dim.TokenResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.AccessToken == "" {
			t.Error("Expected access token, got empty")
		}
		if resp.RefreshToken == "" {
			t.Error("Expected refresh token, got empty")
		}
	})

	t.Run("InvalidCredentials", func(t *testing.T) {
		payload := map[string]string{
			"email":    email,
			"password": "wrongpassword",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 Unauthorized, got %d", rec.Code)
		}
	})
}

func TestAuthHandler_ForgotPassword(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed User
	email := "forgot@example.com"
	SeedUser(t, testDB.DB, "Forgot User", email, "password123")

	t.Run("Success", func(t *testing.T) {
		payload := map[string]string{
			"email": email,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/forgot-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK, got %d", rec.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp["message"] == "" {
			t.Error("Expected message in response")
		}
	})
}

func TestAuthHandler_RefreshTokenEndpoint(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed User and login to get valid tokens
	email := "refresh@example.com"
	password := "password123"
	user := SeedUser(t, testDB.DB, "Refresh User", email, password)

	t.Run("RefreshToken_WithAccessToken_ShouldFail", func(t *testing.T) {
		// Generate access token (typ: at+jwt)
		accessToken := SeedAuth(t, user, &testDB.Config.JWT)

		// Try to use access token on refresh endpoint - should fail
		req := httptest.NewRequest(http.MethodPost, "/api/v1/refresh-token", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// Should return 401 Unauthorized because access token cannot be used as refresh token
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 Unauthorized when using access token for refresh, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("RefreshToken_WithRefreshToken_ShouldSucceed", func(t *testing.T) {
		// Login to get valid refresh token stored in database
		payload := map[string]string{
			"email":    email,
			"password": password,
		}
		body, _ := json.Marshal(payload)

		loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBuffer(body))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()

		router.ServeHTTP(loginRec, loginReq)

		if loginRec.Code != http.StatusOK {
			t.Fatalf("Login failed: %d", loginRec.Code)
		}

		var loginResp dim.TokenResponse
		if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
			t.Fatalf("Failed to decode login response: %v", err)
		}

		// Use valid refresh token - should succeed
		refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/refresh-token", nil)
		refreshReq.Header.Set("Authorization", "Bearer "+loginResp.RefreshToken)
		refreshRec := httptest.NewRecorder()

		router.ServeHTTP(refreshRec, refreshReq)

		if refreshRec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK when using valid refresh token, got %d. Body: %s", refreshRec.Code, refreshRec.Body.String())
		}
	})
}

func TestAuthHandler_ProtectedEndpoint_TokenTypeValidation(t *testing.T) {
	router, _, testDB := SetupApp(t)
	defer testDB.Cleanup()

	// Seed User
	email := "tokentype@example.com"
	password := "password123"
	user := SeedUser(t, testDB.DB, "Token Type User", email, password)

	t.Run("MeEndpoint_WithRefreshToken_ShouldFail", func(t *testing.T) {
		// Generate refresh token (typ: rt+jwt)
		refreshToken := SeedRefreshToken(t, user, &testDB.Config.JWT)

		// Try to use refresh token on protected endpoint - should fail
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// Should return 401 Unauthorized because refresh token cannot be used as access token
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 Unauthorized when using refresh token for /me, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("MeEndpoint_WithAccessToken_ShouldSucceed", func(t *testing.T) {
		// Generate access token (typ: at+jwt)
		accessToken := SeedAuth(t, user, &testDB.Config.JWT)

		// Use access token on protected endpoint - should succeed
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK when using access token for /me, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestAuthHandler_ResetPassword(t *testing.T) {
	router, _, testDB := SetupApp(t) // Need handler to access services
	defer testDB.Cleanup()

	// Seed User
	email := "reset@example.com"
	user := SeedUser(t, testDB.DB, "Reset User", email, "oldpassword")

	// Manually generate reset token using internal service (via handler if accessible, or rebuild service)
	// Since handler fields are private, we can't access authService easily without reflection or changing SetupApp.
	// But wait, SetupApp returns *AppHandler. Fields are private.
	// We can manually init services again with same DB or just use HTTP to get token?
	// ForgotPassword endpoint might return token in Debug mode, but we are in Test mode.
	// Easier way: Directly insert token into DB using dim.TokenStore?
	// Or use dim.AuthService if we init it.

	// Let's initialize a temporary AuthService just for setup
	tokenStore := dim.NewPostgresTokenStore(testDB.DB)
	userStore := NewPostgresUserStore(testDB.DB)
	blocklist := dim.NewPostgresBlocklist(testDB.DB)
	authService, _ := dim.NewAuthService(userStore, tokenStore, blocklist, &testDB.Config.JWT)

	resetToken, err := authService.RequestPasswordReset(context.Background(), email)
	if err != nil {
		t.Fatalf("Failed to generate reset token: %v", err)
	}
	if resetToken == "" {
		t.Fatal("Reset token is empty - User not found or email mismatch")
	}

	// Verify token exists in DB
	// Since hash uses bcrypt (salted), we can't query by hash. We query by user ID.
	var tokenHash string
	err = testDB.DB.QueryRow(context.Background(), "SELECT token_hash FROM password_reset_tokens WHERE user_id = $1", user.ID).Scan(&tokenHash)
	if err != nil {
		t.Fatalf("Failed to query token: %v", err)
	}
	if err := dim.VerifyTokenHash(tokenHash, resetToken); err != nil {
		t.Fatalf("Token hash mismatch: %v", err)
	}

	t.Run("Success", func(t *testing.T) {
		payload := map[string]string{
			"token":    resetToken,
			"password": "NewPassword123!",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/reset-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		// Verify login with new password
		loginPayload := map[string]string{
			"email":    email,
			"password": "NewPassword123!",
		}
		loginBody, _ := json.Marshal(loginPayload)
		loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBuffer(loginBody))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()

		router.ServeHTTP(loginRec, loginReq)

		if loginRec.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK after reset, got %d", loginRec.Code)
		}
	})
}
