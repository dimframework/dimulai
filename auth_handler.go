package dimulai

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dimframework/dim"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	cfg          *dim.Config
	logger       *dim.Logger
	authService  *dim.AuthService
	userStore    *DatabaseUserStore
	emailService *EmailService
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(cfg *dim.Config, logger *dim.Logger, authService *dim.AuthService, userStore *DatabaseUserStore, emailService *EmailService) *AuthHandler {
	return &AuthHandler{
		cfg:          cfg,
		logger:       logger,
		authService:  authService,
		userStore:    userStore,
		emailService: emailService,
	}
}

// RegisterRequest represents the registration payload
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents the login payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dim.BadRequest(w, "Invalid request body", nil)
		return
	}

	v := dim.NewValidator().
		Required("name", req.Name).
		Required("email", req.Email).
		Email("email", req.Email).
		Required("password", req.Password).
		MinLength("password", req.Password, 6)

	if !v.IsValid() {
		dim.BadRequest(w, "Gagal melakukan validasi", v.ErrorMap())
		return
	}

	// Check if user exists
	exists, err := h.userStore.Exists(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to check user existence: %v", err)
		dim.InternalServerError(w, "Failed to check user existence")
		return
	}
	if exists {
		dim.Conflict(w, "Email sudah terdaftar", map[string]string{
			"email": "Email sudah terdaftar",
		})
		return
	}

	// Hash password
	hashedPassword, err := dim.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("Failed to hash password: %v", err)
		dim.InternalServerError(w, "Failed to hash password")
		return
	}

	// Create user
	user := &User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
	}

	if err := h.userStore.Create(r.Context(), user); err != nil {
		h.logger.Error("Failed to create user: %v", err)
		dim.InternalServerError(w, "Failed to create user")
		return
	}

	// Return created user (without password)
	dim.Created(w, user)
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dim.BadRequest(w, "Invalid request body", nil)
		return
	}

	// Use AuthService to login
	accessToken, refreshToken, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		// Handle specific errors if needed, but AuthService returns AppError usually
		if appErr, ok := err.(*dim.AppError); ok {
			dim.JsonAppError(w, appErr)
			return
		}
		dim.Unauthorized(w, "Invalid credentials")
		return
	}

	// Return tokens
	response := dim.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.cfg.JWT.AccessTokenExpiry),
		TokenType:    "Bearer",
	}

	dim.OK(w, response)
}

// RefreshToken handles token rotation
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshToken, ok := dim.GetAuthToken(r)
	if !ok {
		dim.BadRequest(w, "Missing authorization header", nil)
		return
	}

	accessToken, refreshToken, err := h.authService.RefreshToken(r.Context(), refreshToken)
	if err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			dim.JsonAppError(w, appErr)
			return
		}
		dim.Unauthorized(w, "Invalid or expired refresh token")
		return
	}

	response := dim.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.cfg.JWT.AccessTokenExpiry),
		TokenType:    "Bearer",
	}

	dim.OK(w, response)
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	refreshToken, ok := dim.GetAuthToken(r)
	if !ok {
		dim.BadRequest(w, "Missing authorization header", nil)
		return
	}

	if err := h.authService.Logout(r.Context(), refreshToken); err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			dim.JsonAppError(w, appErr)
			return
		}
		dim.InternalServerError(w, "Failed to logout")
		return
	}

	dim.OK(w, map[string]string{"message": "Successfully logged out"})
}

// ForgotPasswordRequest represents the forgot password payload
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPassword handles password reset request
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dim.BadRequest(w, "Invalid request body", nil)
		return
	}

	token, err := h.authService.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			dim.JsonAppError(w, appErr)
			return
		}
		dim.InternalServerError(w, "Failed to process request")
		return
	}

	// Send password reset email
	if token != "" {
		// Get user name for personalization (optional)
		var userName string
		authUser, err := h.userStore.FindByEmail(r.Context(), req.Email)
		if err == nil && authUser != nil {
			// Type assert to get User struct with Name field
			if user, ok := authUser.(*User); ok {
				userName = user.Name
			}
		}

		// Send email asynchronously to not block the response
		// Use WithoutCancel to detach from request context but keep values (trace ID, etc)
		bgCtx := context.WithoutCancel(r.Context())
		go func(ctx context.Context, email, userName, token string) {
			// Set timeout to prevent zombie goroutines if mail server hangs
			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			if err := h.emailService.SendPasswordReset(ctx, email, userName, token); err != nil {
				h.logger.Error("Failed to send password reset email", "error", err, "email", email)
			}
		}(bgCtx, req.Email, userName, token)
	}

	// Always return the same response for security (don't reveal if email exists)
	dim.OK(w, map[string]string{
		"message": "If your email is registered, you will receive a password reset link.",
	})
}

// ResetPasswordRequest represents the reset password payload
type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ResetPassword handles the actual password reset
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dim.BadRequest(w, "Invalid request body", nil)
		return
	}

	if err := h.authService.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			dim.JsonAppError(w, appErr)
			return
		}
		dim.InternalServerError(w, "Failed to reset password")
		return
	}

	dim.OK(w, map[string]string{"message": "Password successfully reset"})
}
