package handlers

import (
	"testing"

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
