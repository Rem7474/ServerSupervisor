// Package user is the application/service layer for user management. It owns the
// user business rules (uniqueness, password policy, role validation, password
// hashing) behind a Repository port, so the logic is unit-testable without a
// database and the HTTP handler only does authz + request/response translation.
package user

import (
	"context"
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally
// (CreateUser keeps the variadic mustChangePassword to match that method exactly).
type Repository interface {
	GetUsers(ctx context.Context) ([]models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	CreateUser(ctx context.Context, username, passwordHash, role string, mustChangePassword ...bool) error
	UpdateUserRole(ctx context.Context, id int64, role string) error
	DeleteUser(ctx context.Context, id int64) error
}

// Service holds the user use-cases.
type Service struct {
	repo Repository
	hash func(string) (string, error) // injectable so tests skip real bcrypt
}

// NewService wires the service with bcrypt password hashing.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, hash: bcryptHash}
}

func bcryptHash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func validRole(role string) bool {
	return role == models.RoleAdmin || role == models.RoleOperator || role == models.RoleViewer
}

// List returns all users (never nil).
func (s *Service) List(ctx context.Context) ([]models.User, error) {
	users, err := s.repo.GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	if users == nil {
		users = []models.User{}
	}
	return users, nil
}

// Create enforces username uniqueness, the password policy and role validity (in
// that order), then stores the user with a bcrypt-hashed password.
func (s *Service) Create(ctx context.Context, username, password, role string) error {
	if _, err := s.repo.GetUserByUsername(ctx, username); err == nil {
		return apperr.Conflict("username already exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if len(password) < 8 {
		return apperr.Validation("password must be at least 8 characters")
	}
	if !validRole(role) {
		return apperr.Validation("invalid role")
	}
	hash, err := s.hash(password)
	if err != nil {
		return err
	}
	return s.repo.CreateUser(ctx, username, hash, role)
}

// UpdateRole validates the role and updates the user.
func (s *Service) UpdateRole(ctx context.Context, id int64, role string) error {
	if !validRole(role) {
		return apperr.Validation("invalid role")
	}
	return s.repo.UpdateUserRole(ctx, id, role)
}

// Delete removes a user by id.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.DeleteUser(ctx, id)
}
