package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"

	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
)

// GenerateTOTPSecret creates a new TOTP secret and returns setup info
func GenerateTOTPSecret(username string) (secret string, qrCodeDataURL string, backupCodes []string, err error) {
	// Generate a new TOTP key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "ServerSupervisor",
		AccountName: username,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate TOTP: %w", err)
	}

	secret = key.Secret()

	// Generate QR Code as data URL
	qr, err := qrcode.New(key.String(), qrcode.Medium)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate QR: %w", err)
	}
	qr.DisableBorder = true

	// Convert to PNG and encode as base64 data URL
	qrImage := qr.Image(256)
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, qrImage); err != nil {
		return "", "", nil, fmt.Errorf("failed to encode QR: %w", err)
	}

	qrCodeDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// Generate 10 backup codes
	backupCodes = generateBackupCodes(10)

	return secret, qrCodeDataURL, backupCodes, nil
}

// VerifyTOTPCode validates a TOTP code against a secret
func VerifyTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// VerifyBackupCode checks if a backup code matches and is valid
func VerifyBackupCode(hashedCodes string, code string) bool {
	var codes []string
	if err := json.Unmarshal([]byte(hashedCodes), &codes); err != nil {
		return false
	}

	for _, hashedCode := range codes {
		if err := bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(code)); err == nil {
			return true
		}
	}
	return false
}

// HashBackupCodes hashes all backup codes for storage
func HashBackupCodes(codes []string) (string, error) {
	var hashed []string
	for _, code := range codes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		hashed = append(hashed, string(hash))
	}
	jsonBytes, err := json.Marshal(hashed)
	return string(jsonBytes), err
}

// generateBackupCodes creates N random alphanumeric backup codes
func generateBackupCodes(count int) []string {
	var codes []string
	for i := 0; i < count; i++ {
		// Generate 8 random bytes and encode as base32, trimmed to 10 chars
		bytes := make([]byte, 8)
		if _, err := rand.Read(bytes); err != nil {
			continue
		}
		code := base32.StdEncoding.EncodeToString(bytes)
		code = code[:10] // Take first 10 chars
		codes = append(codes, code)
	}
	return codes
}
