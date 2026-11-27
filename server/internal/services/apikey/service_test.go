package apikey

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type auditEntry struct{ username, action, ip, details string }

type fakeRepo struct {
	users     map[string]*models.User
	keys      map[int64]*models.APIKey
	auth      map[string]*database.APIKeyAuthRecord
	active    int
	createErr error
	authErr   error
	touchErr  error
	touched   []int64
	audits    []auditEntry
	created   struct {
		prefix, hash, scope string
		expiresAt           *time.Time
	}
	revokedIDs []int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users: map[string]*models.User{
			"alice": {ID: 1, Username: "alice", Role: models.RoleOperator},
			"bob":   {ID: 2, Username: "bob", Role: models.RoleViewer},
		},
		keys: map[int64]*models.APIKey{},
		auth: map[string]*database.APIKeyAuthRecord{},
	}
}

func (f *fakeRepo) GetUserByUsername(_ context.Context, username string) (*models.User, error) {
	if u, ok := f.users[username]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeRepo) CreateAPIKey(_ context.Context, userID int64, maxActive int, name, prefix, keyHash, scope string, expiresAt *time.Time) (*models.APIKey, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.active >= maxActive {
		return nil, database.ErrAPIKeyLimitReached
	}
	f.created.prefix, f.created.hash, f.created.scope, f.created.expiresAt = prefix, keyHash, scope, expiresAt
	k := &models.APIKey{ID: int64(len(f.keys) + 1), Name: name, Prefix: prefix, Scope: scope, ExpiresAt: expiresAt}
	for _, u := range f.users {
		if u.ID == userID {
			k.Username = u.Username
		}
	}
	f.keys[k.ID] = k
	return k, nil
}

func (f *fakeRepo) GetAPIKeyByID(_ context.Context, id int64) (*models.APIKey, error) {
	if k, ok := f.keys[id]; ok {
		return k, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeRepo) ListAPIKeysByUser(_ context.Context, userID int64) ([]models.APIKey, error) {
	out := []models.APIKey{}
	for _, k := range f.keys {
		if f.users[k.Username] != nil && f.users[k.Username].ID == userID {
			out = append(out, *k)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListAllAPIKeys(context.Context) ([]models.APIKey, error) {
	out := []models.APIKey{}
	for _, k := range f.keys {
		out = append(out, *k)
	}
	return out, nil
}

func (f *fakeRepo) RevokeAPIKey(_ context.Context, id int64) (bool, error) {
	k := f.keys[id]
	if k.RevokedAt != nil {
		return false, nil
	}
	now := time.Now()
	k.RevokedAt = &now
	f.revokedIDs = append(f.revokedIDs, id)
	return true, nil
}

func (f *fakeRepo) GetAPIKeyForAuth(_ context.Context, prefix string) (*database.APIKeyAuthRecord, error) {
	if f.authErr != nil {
		return nil, f.authErr
	}
	if r, ok := f.auth[prefix]; ok {
		return r, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeRepo) TouchAPIKeyLastUsed(_ context.Context, id int64) error {
	f.touched = append(f.touched, id)
	return f.touchErr
}

func (f *fakeRepo) CreateAuditLog(_ context.Context, username, action, _, ip, details, _ string) (int64, error) {
	f.audits = append(f.audits, auditEntry{username, action, ip, details})
	return 1, nil
}

var fixedNow = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

func newTestService(repo *fakeRepo) *Service {
	s := NewService(repo)
	s.now = func() time.Time { return fixedNow }
	return s
}

func i18nKey(t *testing.T, err error) string {
	t.Helper()
	var e *apperr.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *apperr.Error, got %T (%v)", err, err)
	}
	return e.I18nKey
}

func TestParseKey(t *testing.T) {
	valid := "ssk_abcdefg2_" + strings.Repeat("A", 43)
	cases := []struct {
		name   string
		raw    string
		prefix string
		ok     bool
	}{
		{"valid", valid, "ssk_abcdefg2", true},
		{"wrong marker", "sk_abcdefg2_" + strings.Repeat("A", 44), "", false},
		{"too short", valid[:len(valid)-1], "", false},
		{"too long", valid + "A", "", false},
		{"missing separator", "ssk_abcdefg2-" + strings.Repeat("A", 43), "", false},
		{"uppercase id", "ssk_ABCDEFG2_" + strings.Repeat("A", 43), "", false},
		{"digit outside base32", "ssk_abcdefg9_" + strings.Repeat("A", 43), "", false},
		{"secret not base64url", "ssk_abcdefg2_" + strings.Repeat("+", 43), "", false},
		{"empty", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix, ok := ParseKey(tc.raw)
			if ok != tc.ok || prefix != tc.prefix {
				t.Fatalf("ParseKey(%q) = (%q, %v), want (%q, %v)", tc.raw, prefix, ok, tc.prefix, tc.ok)
			}
		})
	}
}

func TestHashKey_IsHexSHA256(t *testing.T) {
	// SHA-256("abc") test vector.
	if got := HashKey("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("HashKey(abc) = %s", got)
	}
}

func TestCreate_IssuesParsableKeyAndStoresOnlyItsHash(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo)

	got, err := s.Create(context.Background(), "alice", "10.0.0.1", models.APIKeyCreateRequest{Name: "  grafana  ", Scope: "read", ExpiresInDays: 30})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	prefix, ok := ParseKey(got.Secret)
	if !ok {
		t.Fatalf("issued key %q does not parse", got.Secret)
	}
	if prefix != got.Key.Prefix || prefix != repo.created.prefix {
		t.Fatalf("prefix mismatch: parsed %q, returned %q, stored %q", prefix, got.Key.Prefix, repo.created.prefix)
	}
	if repo.created.hash != HashKey(got.Secret) || strings.Contains(repo.created.hash, got.Secret) {
		t.Fatalf("stored value must be the SHA-256 of the key, got %q", repo.created.hash)
	}
	if got.Key.Name != "grafana" || got.Key.Username != "alice" || repo.created.scope != "read" {
		t.Fatalf("unexpected key: %+v", got.Key)
	}
	if want := fixedNow.Add(30 * 24 * time.Hour); repo.created.expiresAt == nil || !repo.created.expiresAt.Equal(want) {
		t.Fatalf("expires_at = %v, want %v", repo.created.expiresAt, want)
	}
	if len(repo.audits) != 1 || repo.audits[0].action != "api_key_create" || repo.audits[0].ip != "10.0.0.1" {
		t.Fatalf("expected one api_key_create audit entry, got %+v", repo.audits)
	}
	if strings.Contains(repo.audits[0].details, got.Secret) || !strings.Contains(repo.audits[0].details, prefix) {
		t.Fatalf("audit details must name the prefix and never the secret: %q", repo.audits[0].details)
	}
}

func TestCreate_TwoKeysDiffer(t *testing.T) {
	s := newTestService(newFakeRepo())
	a, err := s.Create(context.Background(), "alice", "", models.APIKeyCreateRequest{Name: "a", Scope: "write"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Create(context.Background(), "alice", "", models.APIKeyCreateRequest{Name: "b", Scope: "write"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Secret == b.Secret || a.Key.Prefix == b.Key.Prefix {
		t.Fatal("two keys must not share a secret or a prefix")
	}
}

func TestCreate_NoExpiryWhenZeroDays(t *testing.T) {
	repo := newFakeRepo()
	if _, err := newTestService(repo).Create(context.Background(), "alice", "", models.APIKeyCreateRequest{Name: "ci", Scope: "write"}); err != nil {
		t.Fatal(err)
	}
	if repo.created.expiresAt != nil {
		t.Fatalf("expires_at = %v, want nil", repo.created.expiresAt)
	}
}

func TestCreate_Validation(t *testing.T) {
	cases := []struct {
		name string
		req  models.APIKeyCreateRequest
		key  string
	}{
		{"blank name", models.APIKeyCreateRequest{Name: "   ", Scope: "read"}, apperr.CodeAPIKeyNameInvalid},
		{"name too long", models.APIKeyCreateRequest{Name: strings.Repeat("é", MaxNameLength+1), Scope: "read"}, apperr.CodeAPIKeyNameInvalid},
		{"unknown scope", models.APIKeyCreateRequest{Name: "x", Scope: "admin"}, apperr.CodeAPIKeyScopeInvalid},
		{"negative expiry", models.APIKeyCreateRequest{Name: "x", Scope: "read", ExpiresInDays: -1}, apperr.CodeAPIKeyExpiryInvalid},
		{"expiry too far", models.APIKeyCreateRequest{Name: "x", Scope: "read", ExpiresInDays: MaxExpiresInDays + 1}, apperr.CodeAPIKeyExpiryInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			_, err := newTestService(repo).Create(context.Background(), "alice", "", tc.req)
			if got := i18nKey(t, err); got != tc.key {
				t.Fatalf("i18n key = %q, want %q", got, tc.key)
			}
			if repo.created.prefix != "" {
				t.Fatal("no key may be stored on a validation error")
			}
		})
	}
}

func TestCreate_NameAtMaxLengthAccepted(t *testing.T) {
	_, err := newTestService(newFakeRepo()).Create(context.Background(), "alice", "",
		models.APIKeyCreateRequest{Name: strings.Repeat("é", MaxNameLength), Scope: "read", ExpiresInDays: MaxExpiresInDays})
	if err != nil {
		t.Fatalf("a %d-character name and %d days must be accepted: %v", MaxNameLength, MaxExpiresInDays, err)
	}
}

func TestCreate_LimitReached(t *testing.T) {
	repo := newFakeRepo()
	repo.active = MaxActiveKeysPerUser
	_, err := newTestService(repo).Create(context.Background(), "alice", "", models.APIKeyCreateRequest{Name: "x", Scope: "read"})
	if got := i18nKey(t, err); got != apperr.CodeAPIKeyLimitReached {
		t.Fatalf("i18n key = %q", got)
	}
}

func TestCreate_UnknownUserIsUnauthorized(t *testing.T) {
	_, err := newTestService(newFakeRepo()).Create(context.Background(), "ghost", "", models.APIKeyCreateRequest{Name: "x", Scope: "read"})
	var e *apperr.Error
	if !errors.As(err, &e) || e.HTTPStatus != 401 {
		t.Fatalf("want 401, got %v", err)
	}
}

func TestCreate_PropagatesFailures(t *testing.T) {
	boom := errors.New("boom")

	repo := newFakeRepo()
	repo.createErr = boom
	if _, err := newTestService(repo).Create(context.Background(), "alice", "", models.APIKeyCreateRequest{Name: "x", Scope: "read"}); !errors.Is(err, boom) {
		t.Fatalf("insert error: got %v", err)
	}
	if len(repo.audits) != 0 {
		t.Fatal("a failed creation must not be audited as a success")
	}

	repo = newFakeRepo()
	s := newTestService(repo)
	s.random = func([]byte) (int, error) { return 0, boom }
	_, err := s.Create(context.Background(), "alice", "", models.APIKeyCreateRequest{Name: "x", Scope: "read"})
	var e *apperr.Error
	if !errors.As(err, &e) || e.HTTPStatus != 500 || repo.created.prefix != "" {
		t.Fatalf("entropy failure must be an internal error with nothing stored, got %v", err)
	}
}

func TestListMine(t *testing.T) {
	repo := newFakeRepo()
	repo.keys[1] = &models.APIKey{ID: 1, Username: "alice"}
	repo.keys[2] = &models.APIKey{ID: 2, Username: "bob"}
	s := newTestService(repo)
	keys, err := s.ListMine(context.Background(), "alice")
	if err != nil || len(keys) != 1 || keys[0].ID != 1 {
		t.Fatalf("ListMine(alice) = %+v, %v", keys, err)
	}
	if _, err := s.ListMine(context.Background(), "ghost"); err == nil {
		t.Fatal("unknown user must fail")
	}
	all, err := s.ListAll(context.Background())
	if err != nil || len(all) != 2 {
		t.Fatalf("ListAll = %+v, %v", all, err)
	}
}

func TestRevoke(t *testing.T) {
	setup := func() *fakeRepo {
		repo := newFakeRepo()
		repo.keys[1] = &models.APIKey{ID: 1, Name: "grafana", Prefix: "ssk_aaaaaaaa", Username: "alice"}
		return repo
	}

	t.Run("owner revokes and it is audited", func(t *testing.T) {
		repo := setup()
		if err := newTestService(repo).Revoke(context.Background(), "alice", models.RoleOperator, "10.0.0.2", 1); err != nil {
			t.Fatal(err)
		}
		if len(repo.revokedIDs) != 1 || len(repo.audits) != 1 || repo.audits[0].action != "api_key_revoke" {
			t.Fatalf("revoked=%v audits=%+v", repo.revokedIDs, repo.audits)
		}
	})

	t.Run("admin revokes someone else's key", func(t *testing.T) {
		repo := setup()
		if err := newTestService(repo).Revoke(context.Background(), "root", models.RoleAdmin, "", 1); err != nil {
			t.Fatal(err)
		}
		if repo.audits[0].username != "root" || !strings.Contains(repo.audits[0].details, "owner=alice") {
			t.Fatalf("audit must name the admin as actor and alice as owner: %+v", repo.audits[0])
		}
	})

	t.Run("non-admin cannot see someone else's key", func(t *testing.T) {
		repo := setup()
		err := newTestService(repo).Revoke(context.Background(), "bob", models.RoleOperator, "", 1)
		if got := i18nKey(t, err); got != apperr.CodeAPIKeyNotFound || len(repo.revokedIDs) != 0 {
			t.Fatalf("want not found and nothing revoked, got %q revoked=%v", got, repo.revokedIDs)
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		err := newTestService(setup()).Revoke(context.Background(), "alice", models.RoleAdmin, "", 99)
		if got := i18nKey(t, err); got != apperr.CodeAPIKeyNotFound {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("second revocation is a no-op without audit", func(t *testing.T) {
		repo := setup()
		s := newTestService(repo)
		_ = s.Revoke(context.Background(), "alice", models.RoleOperator, "", 1)
		if err := s.Revoke(context.Background(), "alice", models.RoleOperator, "", 1); err != nil {
			t.Fatal(err)
		}
		if len(repo.audits) != 1 {
			t.Fatalf("want a single audit entry, got %d", len(repo.audits))
		}
	})
}

func TestAuthenticate(t *testing.T) {
	const raw = "ssk_abcdefg2_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	past := fixedNow.Add(-time.Second)
	future := fixedNow.Add(time.Hour)
	record := func(mod func(*database.APIKeyAuthRecord)) *fakeRepo {
		repo := newFakeRepo()
		r := &database.APIKeyAuthRecord{ID: 7, KeyHash: HashKey(raw), Scope: "read", Username: "alice", Role: models.RoleOperator}
		if mod != nil {
			mod(r)
		}
		repo.auth["ssk_abcdefg2"] = r
		return repo
	}

	t.Run("valid key returns the owner's current role and records the use", func(t *testing.T) {
		repo := record(func(r *database.APIKeyAuthRecord) { r.ExpiresAt = &future })
		p, err := newTestService(repo).Authenticate(context.Background(), raw)
		if err != nil {
			t.Fatal(err)
		}
		if *p != (Principal{KeyID: 7, Username: "alice", Role: models.RoleOperator, Scope: "read"}) {
			t.Fatalf("principal = %+v", p)
		}
		if len(repo.touched) != 1 || repo.touched[0] != 7 {
			t.Fatalf("last use not recorded: %v", repo.touched)
		}
	})

	t.Run("failing to record the last use does not reject the key", func(t *testing.T) {
		repo := record(nil)
		repo.touchErr = errors.New("db down")
		if _, err := newTestService(repo).Authenticate(context.Background(), raw); err != nil {
			t.Fatal(err)
		}
	})

	fails := []struct {
		name string
		raw  string
		repo *fakeRepo
		key  string
	}{
		{"malformed", "Bearer nope", record(nil), apperr.CodeAPIKeyInvalid},
		{"unknown prefix", "ssk_zzzzzzzz_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", record(nil), apperr.CodeAPIKeyInvalid},
		{"right prefix, wrong secret", "ssk_abcdefg2_BAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", record(nil), apperr.CodeAPIKeyInvalid},
		{"revoked", raw, record(func(r *database.APIKeyAuthRecord) { r.RevokedAt = &past }), apperr.CodeAPIKeyRevoked},
		{"expired", raw, record(func(r *database.APIKeyAuthRecord) { r.ExpiresAt = &past }), apperr.CodeAPIKeyExpired},
		{"expires exactly now", raw, record(func(r *database.APIKeyAuthRecord) { t := fixedNow; r.ExpiresAt = &t }), apperr.CodeAPIKeyExpired},
	}
	for _, tc := range fails {
		t.Run(tc.name, func(t *testing.T) {
			p, err := newTestService(tc.repo).Authenticate(context.Background(), tc.raw)
			if p != nil {
				t.Fatalf("no principal expected, got %+v", p)
			}
			if got := i18nKey(t, err); got != tc.key {
				t.Fatalf("i18n key = %q, want %q", got, tc.key)
			}
			if len(tc.repo.touched) != 0 {
				t.Fatal("a rejected key must not update last_used_at")
			}
		})
	}

	t.Run("lookup failure is not reported as an invalid key", func(t *testing.T) {
		repo := record(nil)
		repo.authErr = errors.New("db down")
		_, err := newTestService(repo).Authenticate(context.Background(), raw)
		if err == nil || errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("want the repository error, got %v", err)
		}
		var e *apperr.Error
		if errors.As(err, &e) {
			t.Fatalf("a database error must not be turned into a 401: %v", e)
		}
	})
}
