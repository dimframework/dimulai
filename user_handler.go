package dimulai

import (
	"encoding/json"
	"net/http"

	"github.com/dimframework/dim"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userStore *DatabaseUserStore
	logger    *dim.Logger
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userStore *DatabaseUserStore, logger *dim.Logger) *UserHandler {
	return &UserHandler{
		userStore: userStore,
		logger:    logger,
	}
}

// Me handles the current user profile request
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by RequireAuth middleware)
	authUser, ok := dim.GetUser(r)
	if !ok {
		dim.Unauthorized(w, "Unauthorized")
		return
	}

	// Fetch full user profile from database
	user, err := h.userStore.FindByID(r.Context(), authUser.GetID())
	if err != nil {
		h.logger.Error("Failed to find user by ID", "error", err, "id", authUser.GetID())
		dim.NotFound(w, "User not found")
		return
	}

	dim.OK(w, user)
}

// ChangePasswordRequest represents the change password payload
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword handles password change for authenticated user
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	authUser, ok := dim.GetUser(r)
	if !ok {
		dim.Unauthorized(w, "Unauthorized")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dim.BadRequest(w, "Invalid request body", nil)
		return
	}

	v := dim.NewValidator().
		Required("old_password", req.OldPassword).
		Required("new_password", req.NewPassword).
		MinLength("new_password", req.NewPassword, 6)

	if !v.IsValid() {
		dim.BadRequest(w, "Validation failed", v.ErrorMap())
		return
	}

	// Get full user to verify old password
	user, err := h.userStore.FindByID(r.Context(), authUser.GetID())
	if err != nil {
		dim.NotFound(w, "User not found")
		return
	}

	if err := dim.VerifyPassword(user.GetPassword(), req.OldPassword); err != nil {
		dim.Unauthorized(w, "Kata sandi lama salah")
		return
	}

	// Hash new password
	hashedPassword, err := dim.HashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("Failed to hash password", "error", err)
		dim.InternalServerError(w, "Failed to process password")
		return
	}

	// Update password
	// We use type assertion to access fields of struct User
	userStruct, ok := user.(*User)
	if !ok {
		dim.InternalServerError(w, "Invalid user type")
		return
	}

	userStruct.Password = hashedPassword
	if err := h.userStore.Update(r.Context(), userStruct); err != nil {
		h.logger.Error("Failed to update password", "error", err)
		dim.InternalServerError(w, "Failed to update password")
		return
	}

	dim.OK(w, map[string]string{"message": "Password changed successfully"})
}

// UpdateProfileInput represents the profile update payload
type UpdateProfileInput struct {
	Email dim.JsonNull[string] `json:"email"`
	Name  dim.JsonNull[string] `json:"name"`
}

// UpdateProfile handles profile update for authenticated user
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	authUser, ok := dim.GetUser(r)
	if !ok {
		dim.Unauthorized(w, "Unauthorized")
		return
	}

	var req UpdateProfileInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dim.BadRequest(w, "Invalid request body", nil)
		return
	}

	// Validate (check email unique if changed)
	if req.Email.Present && req.Email.Valid {
		if req.Email.Value == "" {
			dim.BadRequest(w, "Email cannot be empty", nil)
			return
		}
		// Check uniqueness
		exists, err := h.userStore.Exists(r.Context(), req.Email.Value)
		if err != nil {
			dim.InternalServerError(w, "Failed to check email")
			return
		}
		// If exists and not belonging to current user
		// We need to check if the existing email belongs to someone else.
		// Exists() returns true if ANY user has it.
		// If the user submits their own email, Exists returns true.
		if exists {
			// Get user by email to check ID
			existingUser, err := h.userStore.FindByEmail(r.Context(), req.Email.Value)
			if err == nil && existingUser.GetID() != authUser.GetID() {
				dim.Conflict(w, "Email already taken", map[string]string{"email": "Email already taken"})
				return
			}
		}
	}

	// Map to UpdateUserRequest
	updateReq := &UpdateUserRequest{
		Email: req.Email,
		Name:  req.Name,
	}

	if err := h.userStore.UpdatePartial(r.Context(), authUser.GetID(), updateReq); err != nil {
		h.logger.Error("Failed to update profile", "error", err)
		dim.InternalServerError(w, "Failed to update profile")
		return
	}

	// Fetch updated user to return
	updatedUser, err := h.userStore.FindByID(r.Context(), authUser.GetID())
	if err != nil {
		dim.InternalServerError(w, "Failed to fetch updated profile")
		return
	}

	dim.OK(w, updatedUser)
}
