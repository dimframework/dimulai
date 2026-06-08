package dimulai

import (
	"context"
	"net/http"
	"time"

	"github.com/dimframework/dim"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	cfg          *AppConfig
	logger       *dim.Logger
	authService  *dim.AuthService
	userStore    *DatabaseUserStore
	emailService *EmailService
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(cfg *AppConfig, logger *dim.Logger, authService *dim.AuthService, userStore *DatabaseUserStore, emailService *EmailService) *AuthHandler {
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
	c := dim.Of(w, r)

	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		c.BadRequest("Invalid request body", nil)
		return
	}

	v := c.Validate().
		Required("name", req.Name).
		Required("email", req.Email).
		Email("email", req.Email).
		Required("password", req.Password).
		MinLength("password", req.Password, 6)

	if !v.IsValid() {
		c.BadRequest("Gagal melakukan validasi", v.ErrorMap())
		return
	}

	exists, err := h.userStore.Exists(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to check user existence", "error", err)
		c.InternalServerError("Failed to check user existence")
		return
	}
	if exists {
		c.Conflict("Email sudah terdaftar", map[string]string{
			"email": "Email sudah terdaftar",
		})
		return
	}

	hashedPassword, err := dim.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("Failed to hash password", "error", err)
		c.InternalServerError("Failed to hash password")
		return
	}

	user := &User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
	}

	if err := h.userStore.Create(r.Context(), user); err != nil {
		h.logger.Error("Failed to create user", "error", err)
		c.InternalServerError("Failed to create user")
		return
	}

	c.Created(user)
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	c := dim.Of(w, r)

	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		c.BadRequest("Invalid request body", nil)
		return
	}

	accessToken, refreshToken, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			c.AppError(appErr)
			return
		}
		c.Unauthorized("Invalid credentials")
		return
	}

	c.OK(dim.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.cfg.JWT.AccessTokenExpiry),
		TokenType:    "Bearer",
	})
}

// RefreshToken handles token rotation
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	c := dim.Of(w, r)

	refreshToken, ok := c.AuthToken()
	if !ok {
		c.BadRequest("Missing authorization header", nil)
		return
	}

	accessToken, newRefreshToken, err := h.authService.RefreshToken(r.Context(), refreshToken)
	if err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			c.AppError(appErr)
			return
		}
		c.Unauthorized("Invalid or expired refresh token")
		return
	}

	c.OK(dim.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int(h.cfg.JWT.AccessTokenExpiry),
		TokenType:    "Bearer",
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	c := dim.Of(w, r)

	refreshToken, ok := c.AuthToken()
	if !ok {
		c.BadRequest("Missing authorization header", nil)
		return
	}

	if err := h.authService.Logout(r.Context(), refreshToken); err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			c.AppError(appErr)
			return
		}
		c.InternalServerError("Failed to logout")
		return
	}

	c.OK(map[string]string{"message": "Successfully logged out"})
}

// ForgotPasswordRequest represents the forgot password payload
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPassword handles password reset request
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	c := dim.Of(w, r)

	var req ForgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		c.BadRequest("Invalid request body", nil)
		return
	}

	token, err := h.authService.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			c.AppError(appErr)
			return
		}
		c.InternalServerError("Failed to process request")
		return
	}

	if token != "" {
		var userName string
		authUser, err := h.userStore.FindByEmail(r.Context(), req.Email)
		if err == nil && authUser != nil {
			if user, ok := authUser.(*User); ok {
				userName = user.Name
			}
		}

		bgCtx := context.WithoutCancel(r.Context())
		go func(ctx context.Context, email, userName, token string) {
			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			if err := h.emailService.SendPasswordReset(ctx, email, userName, token); err != nil {
				h.logger.Error("Failed to send password reset email", "error", err, "email", email)
			}
		}(bgCtx, req.Email, userName, token)
	}

	c.OK(map[string]string{
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
	c := dim.Of(w, r)

	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		c.BadRequest("Invalid request body", nil)
		return
	}

	if err := h.authService.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		if appErr, ok := err.(*dim.AppError); ok {
			c.AppError(appErr)
			return
		}
		c.InternalServerError("Failed to reset password")
		return
	}

	c.OK(map[string]string{"message": "Password successfully reset"})
}
