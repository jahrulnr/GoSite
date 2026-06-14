package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// User represents an admin user stored in SQLite.
type User struct {
	ID        int64
	Name      string
	Email     string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserRepository persists and loads users.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository returns a user repository backed by db.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user and returns the stored record.
func (r *UserRepository) Create(ctx context.Context, user User) (User, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO users (name, email, password, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, user.Name, user.Email, user.Password, now, now)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}

	created, err := r.FindByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	return created, nil
}

// FindByEmail loads a user by email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, email, password, created_at, updated_at
		FROM users WHERE email = ?
	`, email).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &createdAt, &updatedAt)
	if err != nil {
		return User{}, fmt.Errorf("find user by email: %w", err)
	}
	if createdAt.Valid {
		user.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	return user, nil
}

// UpdateProfile updates name, email, and optionally password for a user.
func (r *UserRepository) UpdateProfile(ctx context.Context, id int64, name, email, password string) (User, error) {
	now := time.Now().UTC()
	if password != "" {
		_, err := r.db.ExecContext(ctx, `
			UPDATE users SET name = ?, email = ?, password = ?, updated_at = ?
			WHERE id = ?
		`, name, email, password, now, id)
		if err != nil {
			return User{}, fmt.Errorf("update user profile: %w", err)
		}
	} else {
		_, err := r.db.ExecContext(ctx, `
			UPDATE users SET name = ?, email = ?, updated_at = ?
			WHERE id = ?
		`, name, email, now, id)
		if err != nil {
			return User{}, fmt.Errorf("update user profile: %w", err)
		}
	}
	return r.FindByID(ctx, id)
}

// FindByID loads a user by primary key.
func (r *UserRepository) FindByID(ctx context.Context, id int64) (User, error) {
	var user User
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, email, password, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &createdAt, &updatedAt)
	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}
	if createdAt.Valid {
		user.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	return user, nil
}
