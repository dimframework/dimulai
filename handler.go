package dimulai

import (
	"os"
	"time"

	"github.com/dimframework/dim"
)

type AppHandler struct {
	db        dim.Database
	config    *dim.Config
	router    *dim.Router
	jwtm      *dim.JWTManager
	blocklist *dim.DatabaseBlocklist
	logger    *dim.Logger
}

func NewAppHandler(db dim.Database, config *dim.Config, router *dim.Router, logger *dim.Logger) *AppHandler {
	return &AppHandler{
		db:     db,
		config: config,
		router: router,
		logger: logger,
	}
}

func (h *AppHandler) LoadRouters() error {
	// Global Middleware
	h.router.Use(dim.LoggerMiddleware(h.logger))
	h.router.Use(dim.Recovery(h.logger))

	// Shared dependencies
	userStore := NewDatabaseUserStore(h.db)
	blocklist := dim.NewDatabaseBlocklist(h.db)
	jwtManager, err := dim.NewJWTManager(&h.config.JWT)
	if err != nil {
		h.logger.Error("Failed to init JWT Manager", "error", err)
		return err
	}

	// API v1 Group
	v1 := h.router.Group("/api/v1")

	// Load modules
	if err := h.loadAuthHandler(v1, userStore, blocklist); err != nil {
		return err
	}
	h.loadUserProfile(v1, userStore, jwtManager, blocklist)

	h.router.Build()

	return nil
}

func (h *AppHandler) loadAuthHandler(v1 *dim.RouterGroup, userStore *DatabaseUserStore, blocklist dim.TokenBlocklist) error {
	tokenStore := dim.NewDatabaseTokenStore(h.db)
	rateLimitStore := dim.NewDatabaseRateLimitStore(h.db)

	authService, err := dim.NewAuthService(
		userStore,
		tokenStore,
		blocklist,
		&h.config.JWT,
	)

	if err != nil {
		h.logger.Error("Failed to initialize AuthService", "error", err)
		return err
	}

	// Initialize mailer and email service
	mailer, err := dim.NewMailerFromConfig(&h.config.Email, os.Stdout)
	if err != nil {
		h.logger.Error("Failed to initialize mailer", "error", err)
		return err
	}

	emailService, err := NewEmailService(mailer, &h.config.Email)
	if err != nil {
		h.logger.Error("Failed to initialize email service", "error", err)
		return err
	}

	authHandler := NewAuthHandler(
		h.config,
		h.logger,
		authService,
		userStore,
		emailService,
	)

	// Strict rate limit for auth (e.g. 5 attempts per 15 minutes)
	authRateLimit := dim.RateLimitConfig{
		Enabled:     true,
		PerIP:       5,
		PerUser:     5,
		ResetPeriod: 15 * time.Minute,
	}

	// Public Routes
	v1.Post("/register", authHandler.Register, dim.RateLimit(authRateLimit, rateLimitStore))
	v1.Post("/login", authHandler.Login, dim.RateLimit(authRateLimit, rateLimitStore))
	v1.Post("/forgot-password", authHandler.ForgotPassword, dim.RateLimit(authRateLimit, rateLimitStore))
	v1.Post("/reset-password", authHandler.ResetPassword, dim.RateLimit(authRateLimit, rateLimitStore))

	// Token Management
	v1.Post("/refresh-token", authHandler.RefreshToken, dim.ExpectBearerToken())
	v1.Post("/logout", authHandler.Logout, dim.ExpectBearerToken())

	return nil
}

func (h *AppHandler) loadUserProfile(v1 *dim.RouterGroup, userStore *DatabaseUserStore, jwtManager *dim.JWTManager, blocklist dim.TokenBlocklist) {
	userHandler := NewUserHandler(userStore, h.logger)

	// Protected Routes (Access Token Required & Checked against Blocklist)
	v1.Get("/me", userHandler.Me, dim.RequireAuth(jwtManager, blocklist))
	v1.Put("/me", userHandler.UpdateProfile, dim.RequireAuth(jwtManager, blocklist))
	v1.Put("/me/password", userHandler.ChangePassword, dim.RequireAuth(jwtManager, blocklist))
}
