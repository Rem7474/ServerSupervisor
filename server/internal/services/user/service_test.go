package user

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	existing    *models.User // returned by GetUserByUsername when set
	createdUser string
	createdRole string
	createdHash string
	updatedRole string
	deleted     int64
}

func (f *fakeRepo) GetUsers(context.Context) ([]models.User, error) { return nil, nil }
func (f *fakeRepo) GetUserByUsername(context.Context, string) (*models.User, error) {
	if f.existing != nil {
		return f.existing, nil
	}
	return nil, sql.ErrNoRows
}
func (f *fakeRepo) CreateUser(_ context.Context, username, hash, role string, _ ...bool) error {
	f.createdUser, f.createdHash, f.createdRole = username, hash, role
	return nil
}
func (f *fakeRepo) UpdateUserRole(_ context.Context, _ int64, role string) error {
	f.updatedRole = role
	return nil
}
func (f *fakeRepo) DeleteUser(_ context.Context, id int64) error { f.deleted = id; return nil }

// newService wires a service with a fake hasher so tests don't run real bcrypt.
func newService(repo Repository) *Service {
	s := NewService(repo)
	s.hash = func(p string) (string, error) { return "hashed:" + p, nil }
	return s
}

func TestCreate_HashesAndPersists(t *testing.T) {
	repo := &fakeRepo{}
	if err := newService(repo).Create(context.Background(), "alice", "longenough", models.RoleOperator); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.createdUser != "alice" || repo.createdRole != models.RoleOperator {
		t.Errorf("unexpected created user: %q/%q", repo.createdUser, repo.createdRole)
	}
	if repo.createdHash != "hashed:longenough" {
		t.Errorf("password must be hashed before persisting, got %q", repo.createdHash)
	}
}

func TestCreate_DuplicateUsernameConflict(t *testing.T) {
	repo := &fakeRepo{existing: &models.User{Username: "alice"}}
	err := newService(repo).Create(context.Background(), "alice", "longenough", models.RoleViewer)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 409 {
		t.Fatalf("duplicate username should be apperr 409 (conflict), got %v", err)
	}
	if repo.createdUser != "" {
		t.Error("must not create a user on conflict")
	}
}

func TestCreate_ShortPassword(t *testing.T) {
	err := newService(&fakeRepo{}).Create(context.Background(), "bob", "short", models.RoleViewer)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("short password should be apperr 400, got %v", err)
	}
}

func TestCreate_InvalidRole(t *testing.T) {
	err := newService(&fakeRepo{}).Create(context.Background(), "bob", "longenough", "superadmin")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("invalid role should be apperr 400, got %v", err)
	}
}

// TestCreate_ConflictTakesPrecedence locks in the original validation order:
// a duplicate username is reported even when the password is also too short.
func TestCreate_ConflictTakesPrecedence(t *testing.T) {
	repo := &fakeRepo{existing: &models.User{Username: "alice"}}
	err := newService(repo).Create(context.Background(), "alice", "short", "bogus")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "conflict" {
		t.Fatalf("conflict must take precedence over password/role validation, got %v", err)
	}
}

func TestUpdateRole_Invalid(t *testing.T) {
	err := newService(&fakeRepo{}).UpdateRole(context.Background(), 1, "bogus")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("invalid role should be apperr 400, got %v", err)
	}
}

func TestUpdateRole_Valid(t *testing.T) {
	repo := &fakeRepo{}
	if err := newService(repo).UpdateRole(context.Background(), 1, models.RoleAdmin); err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if repo.updatedRole != models.RoleAdmin {
		t.Errorf("role not updated, got %q", repo.updatedRole)
	}
}
