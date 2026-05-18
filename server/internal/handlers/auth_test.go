package handlers

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword_ProducesVerifiableBcrypt(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "hunter2" {
		t.Fatal("password must not be stored in clear")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("hunter2")); err != nil {
		t.Fatalf("bcrypt comparison failed: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrong")); err == nil {
		t.Fatal("bcrypt accepted the wrong password")
	}
}

func TestHashPassword_DifferentSaltsEachCall(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("bcrypt must produce different hashes for the same password (random salt)")
	}
}

// newTestAuthHandler builds an AuthHandler with only the fields needed by the
// in-memory brute-force path. DB-dependent paths are not exercised here.
func newTestAuthHandler() *AuthHandler {
	return &AuthHandler{memFailures: make(map[string][]time.Time)}
}

func TestMemBruteForce_BlocksAfterThreshold(t *testing.T) {
	h := newTestAuthHandler()
	ip := "10.0.0.1"
	for i := 0; i < bruteForceMaxFails-1; i++ {
		h.memRecordFailure(ip)
		if h.memIsBlocked(ip) {
			t.Fatalf("blocked too early after %d failures", i+1)
		}
	}
	h.memRecordFailure(ip)
	if !h.memIsBlocked(ip) {
		t.Fatal("expected IP to be blocked once threshold is reached")
	}
}

func TestMemBruteForce_OldEntriesEvicted(t *testing.T) {
	h := newTestAuthHandler()
	ip := "10.0.0.2"
	// Inject old failures directly to avoid sleeping for bruteForceWindow.
	old := time.Now().Add(-2 * bruteForceWindow)
	h.memFailures[ip] = []time.Time{old, old, old, old, old, old}
	if h.memIsBlocked(ip) {
		t.Fatal("expired failures must not count as a block")
	}
}

func TestMemBruteForce_PerIPIsolation(t *testing.T) {
	h := newTestAuthHandler()
	for i := 0; i < bruteForceMaxFails; i++ {
		h.memRecordFailure("1.1.1.1")
	}
	if !h.memIsBlocked("1.1.1.1") {
		t.Fatal("attacker IP should be blocked")
	}
	if h.memIsBlocked("2.2.2.2") {
		t.Fatal("unrelated IP must not be blocked")
	}
}

func TestMemBruteForce_ConcurrentRecord(t *testing.T) {
	h := newTestAuthHandler()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			h.memRecordFailure("3.3.3.3")
		}()
	}
	wg.Wait()
	// No assertion on exact count — we mostly want the race detector happy.
	if !h.memIsBlocked("3.3.3.3") {
		t.Fatal("expected concurrent failures to trigger a block")
	}
}
