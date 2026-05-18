package auth

import (
	"testing"
	"time"
)

func TestSignAndVerifyToken(t *testing.T) {
	token := SignToken("user_1", "secret", time.Minute)
	userID, err := VerifyToken(token, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if userID != "user_1" {
		t.Fatalf("expected user_1, got %s", userID)
	}
}
