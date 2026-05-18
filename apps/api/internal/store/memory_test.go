package store

import "testing"

func TestCommitUploadEnforcesQuota(t *testing.T) {
	repo := NewMemoryRepository()
	user, err := repo.CreateUser(User{
		ID:               "user_1",
		Email:            "USER@example.com",
		PasswordVerifier: "verifier",
		QuotaBytes:       10,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.CreateUpload(UploadSession{
		ID:             "upload_1",
		UserID:         user.ID,
		CiphertextSize: 11,
		ObjectKey:      "users/user_1/objects/a",
	})
	if err != ErrQuotaExceeded {
		t.Fatalf("expected quota error, got %v", err)
	}
}

func TestUserEmailIsNormalized(t *testing.T) {
	repo := NewMemoryRepository()
	_, err := repo.CreateUser(User{ID: "user_1", Email: "  USER@example.com ", PasswordVerifier: "v"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.UserByEmail("user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", user.Email)
	}
}
