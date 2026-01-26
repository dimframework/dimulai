package dimulai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimframework/dim"
)

// DatabaseUserStore is the generic SQL implementation of user store
type DatabaseUserStore struct {
	db dim.Database
}

// NewDatabaseUserStore creates a new DatabaseUserStore.
func NewDatabaseUserStore(db dim.Database) *DatabaseUserStore {
	return &DatabaseUserStore{db: db}
}

// Deprecated: Use NewDatabaseUserStore instead
func NewPostgresUserStore(db dim.Database) *DatabaseUserStore {
	return NewDatabaseUserStore(db)
}

// Create membuat user baru dan menyimpannya ke database.
func (s *DatabaseUserStore) Create(ctx context.Context, user *User) error {
	// Generate UUID v7 for sortable ID
	user.ID = dim.NewUuid().String()
	user.CreatedAt = time.Now().UTC().Truncate(time.Second)
	user.UpdatedAt = time.Now().UTC().Truncate(time.Second)

	query := s.db.Rebind(`INSERT INTO users (id, email, name, password, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at, updated_at`)

	err := s.db.QueryRow(ctx, query,
		user.ID,
		user.Email,
		user.Name,
		user.Password,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// FindByID mencari user berdasarkan ID.
func (s *DatabaseUserStore) FindByID(ctx context.Context, id string) (dim.Authenticatable, error) {
	user := &User{}
	query := s.db.Rebind(`SELECT id, email, name, password, created_at, updated_at
		 FROM users WHERE id = $1`)
	err := s.db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	return user, nil
}

// FindByEmail mencari user berdasarkan email address.
func (s *DatabaseUserStore) FindByEmail(ctx context.Context, email string) (dim.Authenticatable, error) {
	user := &User{}
	query := s.db.Rebind(`SELECT id, email, name, password, created_at, updated_at
		 FROM users WHERE email = $1`)
	err := s.db.QueryRow(ctx, query, email).Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	return user, nil
}

// Update mengupdate semua field user.
func (s *DatabaseUserStore) Update(ctx context.Context, u dim.Authenticatable) error {
	user, ok := u.(*User)
	if !ok {
		return fmt.Errorf("invalid user type: expected *User")
	}

	user.UpdatedAt = time.Now().UTC().Truncate(time.Second)

	query := s.db.Rebind(`UPDATE users SET email = $1, name = $2, password = $3, updated_at = $4
		 WHERE id = $5`)
	err := s.db.Exec(ctx, query,
		user.Email,
		user.Name,
		user.Password,
		user.UpdatedAt,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdatePartial melakukan partial update user fields.
func (s *DatabaseUserStore) UpdatePartial(ctx context.Context, id string, req *UpdateUserRequest) error {
	var setClauses []string
	var args []interface{}
	argIndex := 1

	// Check each field and add to SET clause if present
	if req.Email.Present && req.Email.Valid {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, req.Email.Value)
		argIndex++
	}

	if req.Name.Present && req.Name.Valid {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, req.Name.Value)
		argIndex++
	}

	if req.Password.Present && req.Password.Valid {
		// Hash password before storing
		hashedPassword, err := dim.HashPassword(req.Password.Value)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("password = $%d", argIndex))
		args = append(args, hashedPassword)
		argIndex++
	}

	if len(setClauses) == 0 {
		return nil // Nothing to update
	}

	// Always update updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC().Truncate(time.Second))
	argIndex++

	// Add WHERE id clause
	args = append(args, id)

	// Build final query
	query := fmt.Sprintf(
		"UPDATE users SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "),
		argIndex,
	)

	err := s.db.Exec(ctx, s.db.Rebind(query), args...)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// Delete menghapus user dari database.
func (s *DatabaseUserStore) Delete(ctx context.Context, id string) error {
	query := s.db.Rebind("DELETE FROM users WHERE id = $1")
	err := s.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// Exists mengecek apakah user dengan email tertentu sudah ada.
func (s *DatabaseUserStore) Exists(ctx context.Context, email string) (bool, error) {
	var exists bool
	query := s.db.Rebind("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)")
	err := s.db.QueryRow(ctx, query,
		email,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}

	return exists, nil
}

// MockUserStore is a mock implementation for testing
type MockUserStore struct {
	users map[string]*User
}

// NewMockUserStore membuat mock user store untuk testing.
func NewMockUserStore() *MockUserStore {
	return &MockUserStore{
		users: make(map[string]*User),
	}
}

// Create membuat user baru dalam mock store.
func (s *MockUserStore) Create(ctx context.Context, user *User) error {
	user.ID = dim.NewUuid().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	s.users[user.ID] = user
	return nil
}

// FindByID mencari user berdasarkan ID dalam mock store.
func (s *MockUserStore) FindByID(ctx context.Context, id string) (*User, error) {
	if user, exists := s.users[id]; exists {
		return user, nil
	}
	return nil, fmt.Errorf("user not found")
}

// FindByEmail mencari user berdasarkan email dalam mock store.
func (s *MockUserStore) FindByEmail(ctx context.Context, email string) (*User, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// Update mengupdate user dalam mock store.
func (s *MockUserStore) Update(ctx context.Context, user *User) error {
	if _, exists := s.users[user.ID]; !exists {
		return fmt.Errorf("user not found")
	}
	user.UpdatedAt = time.Now()
	s.users[user.ID] = user
	return nil
}

// Delete menghapus user dari mock store.
func (s *MockUserStore) Delete(ctx context.Context, id string) error {
	delete(s.users, id)
	return nil
}

// Exists mengecek apakah user dengan email tertentu ada dalam mock store.
func (s *MockUserStore) Exists(ctx context.Context, email string) (bool, error) {
	for _, user := range s.users {
		if user.Email == email {
			return true, nil
		}
	}
	return false, nil
}

// UpdatePartial melakukan partial update user fields dalam mock store.
func (s *MockUserStore) UpdatePartial(ctx context.Context, id string, req *UpdateUserRequest) error {
	user, exists := s.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Update email if present and valid
	if req.Email.Present && req.Email.Valid {
		user.Email = req.Email.Value
	}

	// Update name if present and valid
	if req.Name.Present && req.Name.Valid {
		user.Name = req.Name.Value
	}

	// Update password if present and valid
	if req.Password.Present && req.Password.Valid {
		hashedPassword, err := dim.HashPassword(req.Password.Value)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.Password = hashedPassword
	}

	user.UpdatedAt = time.Now()
	return nil
}
