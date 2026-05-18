package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"private-cloud-storage/apps/api/internal/config"
	"private-cloud-storage/apps/api/internal/objectstore"
	"private-cloud-storage/apps/api/internal/store"
)

func TestSignupLoginAndFolderFlowKeepsOpaqueMetadata(t *testing.T) {
	api := NewServer(config.Config{
		TokenSecret:    "secret",
		DefaultQuotaGB: 1,
		UploadTTL:      time.Minute,
		DownloadTTL:    time.Minute,
	}, store.NewMemoryRepository(), objectstore.NewS3Presigner(objectstore.S3Config{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "private-cloud",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		PathStyle:       true,
	}))

	token := postJSONForToken(t, api.Routes(), "/v1/auth/signup", map[string]any{
		"email":            "user@example.com",
		"passwordVerifier": "verifier",
		"keyBundle": map[string]string{
			"passwordWrappedMasterKey":       "wrapped-password",
			"passwordKdfSalt":                "salt",
			"recoveryPhraseWrappedMasterKey": "wrapped-phrase",
			"recoveryFileWrappedMasterKey":   "wrapped-file",
		},
	})

	reqBody, _ := json.Marshal(map[string]string{"encryptedMetadata": "opaque-name-ciphertext"})
	req := httptest.NewRequest(http.MethodPost, "/v1/files", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("plaintext")) {
		t.Fatal("response leaked plaintext marker")
	}
}

func postJSONForToken(t *testing.T, handler http.Handler, path string, body any) string {
	t.Helper()
	reqBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Token == "" {
		t.Fatal("missing token")
	}
	return response.Token
}
