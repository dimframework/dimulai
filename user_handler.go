package dimulai

import (
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
	c := dim.Of(w, r)

	authUser, ok := c.User()
	if !ok {
		c.Unauthorized("Unauthorized")
		return
	}

	user, err := h.userStore.FindByID(r.Context(), authUser.GetID())
	if err != nil {
		h.logger.Error("Failed to find user by ID", "error", err, "id", authUser.GetID())
		c.NotFound("User not found")
		return
	}

	c.OK(user)
}

// ChangePasswordRequest represents the change password payload
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword handles password change for authenticated user
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	c := dim.Of(w, r)

	authUser, ok := c.User()
	if !ok {
		c.Unauthorized("Unauthorized")
		return
	}

	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		c.BadRequest("Invalid request body", nil)
		return
	}

	v := c.Validate().
		Required("old_password", req.OldPassword).
		Required("new_password", req.NewPassword).
		MinLength("new_password", req.NewPassword, 6)

	if !v.IsValid() {
		c.BadRequest("Validation failed", v.ErrorMap())
		return
	}

	user, err := h.userStore.FindByID(r.Context(), authUser.GetID())
	if err != nil {
		c.NotFound("User not found")
		return
	}

	if err := dim.VerifyPassword(user.GetPassword(), req.OldPassword); err != nil {
		c.Unauthorized("Kata sandi lama salah")
		return
	}

	hashedPassword, err := dim.HashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("Failed to hash password", "error", err)
		c.InternalServerError("Failed to process password")
		return
	}

	userStruct, ok := user.(*User)
	if !ok {
		c.InternalServerError("Invalid user type")
		return
	}

	userStruct.Password = hashedPassword
	if err := h.userStore.Update(r.Context(), userStruct); err != nil {
		h.logger.Error("Failed to update password", "error", err)
		c.InternalServerError("Failed to update password")
		return
	}

	c.OK(map[string]string{"message": "Password changed successfully"})
}

// UpdateProfileInput represents the profile update payload
type UpdateProfileInput struct {
	Email dim.JsonNull[string] `json:"email"`
	Name  dim.JsonNull[string] `json:"name"`
}

// UpdateProfile handles profile update for authenticated user
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	c := dim.Of(w, r)

	authUser, ok := c.User()
	if !ok {
		c.Unauthorized("Unauthorized")
		return
	}

	var req UpdateProfileInput
	if err := c.Bind(&req); err != nil {
		c.BadRequest("Invalid request body", nil)
		return
	}

	if req.Email.Present && req.Email.Valid {
		if req.Email.Value == "" {
			c.BadRequest("Email cannot be empty", nil)
			return
		}
		exists, err := h.userStore.Exists(r.Context(), req.Email.Value)
		if err != nil {
			c.InternalServerError("Failed to check email")
			return
		}
		if exists {
			existingUser, err := h.userStore.FindByEmail(r.Context(), req.Email.Value)
			if err == nil && existingUser.GetID() != authUser.GetID() {
				c.Conflict("Email already taken", map[string]string{"email": "Email already taken"})
				return
			}
		}
	}

	updateReq := &UpdateUserRequest{
		Email: req.Email,
		Name:  req.Name,
	}

	if err := h.userStore.UpdatePartial(r.Context(), authUser.GetID(), updateReq); err != nil {
		h.logger.Error("Failed to update profile", "error", err)
		c.InternalServerError("Failed to update profile")
		return
	}

	updatedUser, err := h.userStore.FindByID(r.Context(), authUser.GetID())
	if err != nil {
		c.InternalServerError("Failed to fetch updated profile")
		return
	}

	c.OK(updatedUser)
}
