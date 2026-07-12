package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTPSecret(t *testing.T) {
	secret, qrDataURL, codes, err := GenerateTOTPSecret("alice")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret returned error: %v", err)
	}
	if secret == "" {
		t.Error("expected a non-empty secret")
	}
	if !strings.HasPrefix(qrDataURL, "data:image/png;base64,") {
		t.Errorf("qrCodeDataURL does not start with the expected data URL prefix: %q", qrDataURL[:min(40, len(qrDataURL))])
	}
	if len(codes) != 10 {
		t.Fatalf("got %d backup codes, want 10", len(codes))
	}
	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if len(c) != 10 {
			t.Errorf("backup code %q has length %d, want 10", c, len(c))
		}
		if seen[c] {
			t.Errorf("duplicate backup code %q", c)
		}
		seen[c] = true
	}
}

func TestVerifyTOTPCode(t *testing.T) {
	secret, _, _, err := GenerateTOTPSecret("bob")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret returned error: %v", err)
	}

	validCode, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate a code for the test secret: %v", err)
	}
	if !VerifyTOTPCode(secret, validCode) {
		t.Error("VerifyTOTPCode rejected a freshly generated valid code")
	}

	// A code generated 10 minutes ago is well outside any reasonable skew
	// window, so this is a deterministic (non-flaky) negative case, unlike
	// asserting against a fixed string which could in principle collide.
	staleCode, err := totp.GenerateCode(secret, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("failed to generate a stale code for the test secret: %v", err)
	}
	if VerifyTOTPCode(secret, staleCode) {
		t.Error("VerifyTOTPCode accepted a code generated 10 minutes ago")
	}

	if VerifyTOTPCode("", validCode) {
		t.Error("VerifyTOTPCode accepted a code against an empty secret")
	}
}

func TestHashAndVerifyBackupCode(t *testing.T) {
	codes := []string{"AAAAAAAAAA", "BBBBBBBBBB", "CCCCCCCCCC"}
	hashed, err := HashBackupCodes(codes)
	if err != nil {
		t.Fatalf("HashBackupCodes returned error: %v", err)
	}

	for _, c := range codes {
		if !VerifyBackupCode(hashed, c) {
			t.Errorf("VerifyBackupCode rejected valid backup code %q", c)
		}
	}
	if VerifyBackupCode(hashed, "not-a-real-code") {
		t.Error("VerifyBackupCode accepted a code that was never issued")
	}
}

func TestVerifyBackupCode_MalformedOrEmptyStorage(t *testing.T) {
	if VerifyBackupCode("not valid json", "AAAAAAAAAA") {
		t.Error("VerifyBackupCode accepted a code against malformed stored JSON")
	}
	if VerifyBackupCode("", "AAAAAAAAAA") {
		t.Error("VerifyBackupCode accepted a code against an empty stored value")
	}
	if VerifyBackupCode("[]", "AAAAAAAAAA") {
		t.Error("VerifyBackupCode accepted a code against an empty code list")
	}
}

func TestGenerateBackupCodes_Count(t *testing.T) {
	codes := generateBackupCodes(5)
	if len(codes) != 5 {
		t.Fatalf("got %d codes, want 5", len(codes))
	}
	for _, c := range codes {
		if len(c) != 10 {
			t.Errorf("backup code %q has length %d, want 10", c, len(c))
		}
	}
}
