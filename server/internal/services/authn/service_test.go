package authn

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	user        *models.User
	getUserErr  error
	failCount   int
	pwUpdated   string
	loginEvents []bool
}

func (f *fakeRepo) GetUserByUsername(context.Context, string) (*models.User, error) {
	return f.user, f.getUserErr
}
func (f *fakeRepo) GetUserByID(context.Context, int64) (*models.User, error) {
	return f.user, f.getUserErr
}
func (f *fakeRepo) CreateLoginEvent(_ context.Context, _, _, _ string, success bool) error {
	f.loginEvents = append(f.loginEvents, success)
	return nil
}
func (f *fakeRepo) CountRecentFailedLoginsAfterUnblock(context.Context, string, time.Time) (int, error) {
	return f.failCount, nil
}
func (f *fakeRepo) ConsumeMFABackupCode(context.Context, string, string) error { return nil }
func (f *fakeRepo) CreateRefreshToken(context.Context, int64, string, time.Time) error {
	return nil
}
func (f *fakeRepo) RotateRefreshToken(context.Context, string, string, time.Time) (int64, error) {
	return 1, nil
}
func (f *fakeRepo) RevokeRefreshToken(context.Context, string) error             { return nil }
func (f *fakeRepo) RevokeAllOtherSessions(context.Context, string, string) error { return nil }
func (f *fakeRepo) UpdateUserPassword(_ context.Context, _, hash string) error {
	f.pwUpdated = hash
	return nil
}
func (f *fakeRepo) SetUserTOTPSecret(context.Context, int64, string, string, bool) error {
	return nil
}
func (f *fakeRepo) DisableUserMFA(context.Context, string) error { return nil }
func (f *fakeRepo) GetLoginStats(context.Context, time.Time) (*models.LoginStats, error) {
	return &models.LoginStats{}, nil
}
func (f *fakeRepo) GetTopFailedIPs(context.Context, time.Time, int) ([]models.IPFailCount, error) {
	return nil, nil
}
func (f *fakeRepo) GetCurrentlyBlockedIPs(context.Context, time.Time, int) ([]string, error) {
	return nil, nil
}
func (f *fakeRepo) UpsertIPUnblock(context.Context, string, string) error { return nil }
func (f *fakeRepo) CreateAuditLog(context.Context, string, string, string, string, string, string) (int64, error) {
	return 1, nil
}
func (f *fakeRepo) GetLoginEventsByUser(context.Context, string, int, int) ([]models.LoginEvent, error) {
	return nil, nil
}
func (f *fakeRepo) GetAllLoginEvents(context.Context, int, int) ([]models.LoginEvent, error) {
	return nil, nil
}
func (f *fakeRepo) CountLoginEvents(context.Context) (int64, error) { return 0, nil }

func userWithPassword(t *testing.T, password string) *models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return &models.User{ID: 1, Username: "alice", Role: "viewer", PasswordHash: string(hash)}
}

func newSvc(repo Repository) *Service {
	return NewService(repo, &config.Config{JWTSecret: "test", JWTExpiration: time.Hour, RefreshTokenExpiration: time.Hour})
}

func httpStatus(err error) int {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.HTTPStatus
	}
	return 0
}

func TestAuthenticate_Blocked(t *testing.T) {
	svc := newSvc(&fakeRepo{failCount: bruteForceMaxFails})
	_, _, err := svc.Authenticate(context.Background(), "alice", "x", "", "1.2.3.4", "ua")
	if httpStatus(err) != 429 {
		t.Fatalf("blocked IP should be 429, got %v", err)
	}
}

func TestAuthenticate_BadPassword(t *testing.T) {
	svc := newSvc(&fakeRepo{user: userWithPassword(t, "correct")})
	_, _, err := svc.Authenticate(context.Background(), "alice", "wrong", "", "1.2.3.4", "ua")
	if httpStatus(err) != 401 {
		t.Fatalf("bad password should be 401, got %v", err)
	}
}

func TestAuthenticate_RequiresMFA(t *testing.T) {
	u := userWithPassword(t, "correct")
	u.MFAEnabled = true
	u.TOTPSecret = "SECRET"
	_, requireMFA, err := newSvc(&fakeRepo{user: u}).Authenticate(context.Background(), "alice", "correct", "", "1.2.3.4", "ua")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !requireMFA {
		t.Error("expected requireMFA=true when MFA enabled and no code supplied")
	}
}

func TestAuthenticate_Success(t *testing.T) {
	repo := &fakeRepo{user: userWithPassword(t, "correct")}
	user, requireMFA, err := newSvc(repo).Authenticate(context.Background(), "alice", "correct", "", "1.2.3.4", "ua")
	if err != nil || requireMFA || user == nil {
		t.Fatalf("expected success, got user=%v mfa=%v err=%v", user, requireMFA, err)
	}
	if len(repo.loginEvents) != 1 || !repo.loginEvents[0] {
		t.Errorf("expected one successful login event, got %v", repo.loginEvents)
	}
}

func TestChangePassword(t *testing.T) {
	repo := &fakeRepo{user: userWithPassword(t, "current")}
	svc := newSvc(repo)
	if err := svc.ChangePassword(context.Background(), "alice", "current", "short"); httpStatus(err) != 400 {
		t.Errorf("short new password should be 400, got %v", err)
	}
	if err := svc.ChangePassword(context.Background(), "alice", "wrong", "longenough"); httpStatus(err) != 401 {
		t.Errorf("wrong current password should be 401, got %v", err)
	}
	if err := svc.ChangePassword(context.Background(), "alice", "current", "longenough"); err != nil {
		t.Fatalf("valid change: %v", err)
	}
	if repo.pwUpdated == "" {
		t.Error("password hash should have been persisted")
	}
}

func TestIssueSession(t *testing.T) {
	tokens, err := newSvc(&fakeRepo{}).IssueSession(context.Background(), &models.User{ID: 1, Username: "alice", Role: "viewer"})
	if err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.CSRFToken == "" {
		t.Error("session tokens must all be populated")
	}
}

// ===== in-memory brute-force fallback (DB-outage path) =====

func TestMemBruteForce_BlocksAfterThreshold(t *testing.T) {
	s := newSvc(&fakeRepo{})
	ip := "10.0.0.1"
	for i := 0; i < bruteForceMaxFails-1; i++ {
		s.memRecordFailure(ip)
		if s.memIsBlocked(ip) {
			t.Fatalf("blocked too early after %d failures", i+1)
		}
	}
	s.memRecordFailure(ip)
	if !s.memIsBlocked(ip) {
		t.Fatal("expected IP to be blocked once threshold is reached")
	}
}

func TestMemBruteForce_OldEntriesEvicted(t *testing.T) {
	s := newSvc(&fakeRepo{})
	ip := "10.0.0.2"
	old := time.Now().Add(-2 * bruteForceWindow)
	s.memFailures[ip] = []time.Time{old, old, old, old, old, old}
	if s.memIsBlocked(ip) {
		t.Fatal("expired failures must not count as a block")
	}
}

func TestMemBruteForce_PerIPIsolation(t *testing.T) {
	s := newSvc(&fakeRepo{})
	for i := 0; i < bruteForceMaxFails; i++ {
		s.memRecordFailure("1.1.1.1")
	}
	if !s.memIsBlocked("1.1.1.1") {
		t.Fatal("attacker IP should be blocked")
	}
	if s.memIsBlocked("2.2.2.2") {
		t.Fatal("unrelated IP must not be blocked")
	}
}

func TestMemBruteForce_ConcurrentRecord(t *testing.T) {
	s := newSvc(&fakeRepo{})
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			s.memRecordFailure("3.3.3.3")
		}()
	}
	wg.Wait()
	if !s.memIsBlocked("3.3.3.3") {
		t.Fatal("expected concurrent failures to trigger a block")
	}
}
