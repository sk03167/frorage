package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"frorage/apps/api/internal/config"
	"frorage/apps/api/internal/objectstore"
	"frorage/apps/api/internal/store"
)

func TestSignupLoginAndFolderFlowKeepsOpaqueMetadata(t *testing.T) {
	api := NewServer(config.Config{
		TokenSecret:               "secret",
		AdminToken:                "admin",
		MasterKeyEncryptionSecret: "master",
		DefaultQuotaGB:            1,
		UploadTTL:                 time.Minute,
		DownloadTTL:               time.Minute,
	}, store.NewMemoryRepository(), objectstore.NewMemoryStore())

	token := postJSONForToken(t, api.Routes(), "/v1/auth/signup", map[string]any{
		"email":            "user@example.com",
		"passwordVerifier": "verifier",
	})

	reqBody, _ := json.Marshal(map[string]string{"name": "folder"})
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

func TestServerEncryptsUploadedBytesAndDecryptsDownload(t *testing.T) {
	objects := objectstore.NewMemoryStore()
	api := NewServer(config.Config{
		TokenSecret:               "secret",
		AdminToken:                "admin",
		MasterKeyEncryptionSecret: "master",
		DefaultQuotaGB:            1,
		UploadTTL:                 time.Minute,
		DownloadTTL:               time.Minute,
	}, store.NewMemoryRepository(), objects)

	token := postJSONForToken(t, api.Routes(), "/v1/auth/signup", map[string]any{
		"email":            "user@example.com",
		"passwordVerifier": "verifier",
	})

	var body bytes.Buffer
	body.WriteString("--frorage\r\n")
	body.WriteString(`Content-Disposition: form-data; name="name"` + "\r\n\r\n")
	body.WriteString("note.txt\r\n")
	body.WriteString("--frorage\r\n")
	body.WriteString(`Content-Disposition: form-data; name="mimeType"` + "\r\n\r\n")
	body.WriteString("text/plain\r\n")
	body.WriteString("--frorage\r\n")
	body.WriteString(`Content-Disposition: form-data; name="file"; filename="note.txt"` + "\r\n")
	body.WriteString("Content-Type: text/plain\r\n\r\n")
	body.WriteString("secret file")
	body.WriteString("\r\n--frorage--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/v1/files/upload", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=frorage")
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var file store.FileRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &file); err != nil {
		t.Fatal(err)
	}
	if file.ObjectKey != nil {
		t.Fatal("normal upload response exposed object key")
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/files/"+file.ID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "secret file" {
		t.Fatalf("expected decrypted file, got %q", rec.Body.String())
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
