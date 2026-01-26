package dimulai

import (
	"time"

	"github.com/dimframework/dim"
)

// User represents a user entity
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetID returns the user ID as string (implements dim.Authenticatable)
func (u *User) GetID() string {
	return u.ID
}

// GetEmail returns the user email (implements dim.Authenticatable)
func (u *User) GetEmail() string {
	return u.Email
}

// GetPassword returns the user password hash (implements dim.Authenticatable)
func (u *User) GetPassword() string {
	return u.Password
}

// SetPassword sets the user password hash (implements dim.Authenticatable)
func (u *User) SetPassword(password string) {
	u.Password = password
}

// UpdateUserRequest represents a partial update request for a user
// Fields use JsonNull to distinguish between:
// - Not sent (don't update)
// - Sent as null (clear field - if applicable)
// - Sent with value (update to new value)
type UpdateUserRequest struct {
	Email    dim.JsonNull[string] `json:"email"`
	Name     dim.JsonNull[string] `json:"name"`
	Password dim.JsonNull[string] `json:"password"`
}
