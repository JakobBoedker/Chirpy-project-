package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "my-test-secret"

	token, err := MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT returned an error: %v", err)
	}

	gotID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT returned an error: %v", err)
	}

	if gotID != userID {
		t.Errorf("expected userID %v, got %v", userID, gotID)
	}

}

func TestExpiredJWT(t *testing.T) {
	userID := uuid.New()
	secret := "my-test-secret"

	token, err := MakeJWT(userID, secret, -1*time.Second)
	if err != nil {
		t.Fatalf("MakeJWT returned an error: %v", err)
	}

	if _, err := ValidateJWT(token, secret); err == nil {
		t.Error("expected an error for an expired token, got nil")
	}

}

func TestWrongSecretJWT(t *testing.T) {
	userID := uuid.New()
	token, err := MakeJWT(userID, "secret-a", time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT returned an error: %v", err)
	}

	if _, err := ValidateJWT(token, "secret-b"); err == nil {
		t.Error("expected an error for a wrong secret, got nil")
	}
}
