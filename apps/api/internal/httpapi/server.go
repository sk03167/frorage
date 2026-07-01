package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"frorage/apps/api/internal/auth"
	"frorage/apps/api/internal/billing"
	"frorage/apps/api/internal/config"
	"frorage/apps/api/internal/cryptoutil"
	"frorage/apps/api/internal/objectstore"
	"frorage/apps/api/internal/store"
)

type Server struct {
	cfg     config.Config
	repo    store.Repository
	objects objectstore.ObjectStore
	mux     *http.ServeMux
}

func NewServer(cfg config.Config, repo store.Repository, objects objectstore.ObjectStore) *Server {
	server := &Server{cfg: cfg, repo: repo, objects: objects, mux: http.NewServeMux()}
	server.mount()
	return server
}

func (s *Server) Routes() http.Handler {
	return withCORS(s.mux)
}

func (s *Server) mount() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("POST /v1/auth/signup", s.signup)
	s.mux.HandleFunc("POST /v1/auth/login", s.login)
	s.mux.HandleFunc("POST /v1/auth/forgot-password", s.forgotPassword)
	s.mux.HandleFunc("POST /v1/auth/reset-password", s.resetPassword)
	s.mux.HandleFunc("POST /v1/uploads/init", s.requireAuth(s.initUpload))
	s.mux.HandleFunc("POST /v1/uploads/{uploadId}/commit", s.requireAuth(s.commitUpload))
	s.mux.HandleFunc("GET /v1/files", s.requireAuth(s.listFiles))
	s.mux.HandleFunc("POST /v1/files", s.requireAuth(s.createFolder))
	s.mux.HandleFunc("POST /v1/files/upload", s.requireAuth(s.uploadFile))
	s.mux.HandleFunc("PATCH /v1/files/{fileId}", s.requireAuth(s.updateFile))
	s.mux.HandleFunc("DELETE /v1/files/{fileId}", s.requireAuth(s.deleteFile))
	s.mux.HandleFunc("POST /v1/files/{fileId}/download", s.requireAuth(s.downloadFile))
	s.mux.HandleFunc("GET /v1/files/{fileId}/preview", s.requireAuth(s.previewFile))
	s.mux.HandleFunc("GET /v1/admin/users", s.requireAdmin(s.adminUsers))
	s.mux.HandleFunc("GET /v1/admin/users/{userId}/files", s.requireAdmin(s.adminFiles))
	s.mux.HandleFunc("GET /v1/admin/files/{fileId}/preview", s.requireAdmin(s.adminPreviewFile))
	s.mux.HandleFunc("POST /v1/admin/files/{fileId}/download", s.requireAdmin(s.adminDownloadFile))
	s.mux.HandleFunc("GET /v1/billing/usage", s.requireAuth(s.usage))
}

type signupRequest struct {
	Email            string `json:"email"`
	PasswordVerifier string `json:"passwordVerifier"`
}

type loginRequest struct {
	Email            string `json:"email"`
	PasswordVerifier string `json:"passwordVerifier"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Email == "" || req.PasswordVerifier == "" {
		writeError(w, http.StatusBadRequest, "email and password verifier are required")
		return
	}
	accountKey, err := cryptoutil.NewAccountKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	encryptedKey, err := cryptoutil.EncryptKey(s.rootKey(), accountKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now().UTC()
	userID := store.NewID("user")
	user, err := s.repo.CreateUser(store.User{
		ID:                        userID,
		Email:                     req.Email,
		PasswordVerifier:          req.PasswordVerifier,
		StoragePrefix:             "users/" + userID,
		EncryptedAccountMasterKey: encryptedKey.Ciphertext,
		AccountMasterKeyNonce:     encryptedKey.Nonce,
		QuotaBytes:                s.cfg.DefaultQuotaGB * 1024 * 1024 * 1024,
		CreatedAt:                 now,
	})
	if err != nil {
		statusForError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.authResponse(user))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.repo.UserByEmail(req.Email)
	if err != nil || user.PasswordVerifier != req.PasswordVerifier {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	writeJSON(w, http.StatusOK, s.authResponse(user))
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.repo.UserByEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	reset, err := s.repo.CreatePasswordReset(store.PasswordReset{
		Token:     store.NewID("reset"),
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	})
	if err != nil {
		statusForError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"resetToken": reset.Token})
}

type resetPasswordRequest struct {
	Token            string `json:"token"`
	PasswordVerifier string `json:"passwordVerifier"`
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	reset, err := s.repo.PasswordResetByToken(req.Token)
	if err != nil || reset.Used || time.Now().UTC().After(reset.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "invalid or expired reset token")
		return
	}
	user, err := s.repo.UpdatePasswordVerifier(reset.UserID, req.PasswordVerifier)
	if err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.MarkPasswordResetUsed(req.Token)
	writeJSON(w, http.StatusOK, s.authResponse(user))
}

func (s *Server) authResponse(user store.User) authResponse {
	return authResponse{
		Token: auth.SignToken(user.ID, s.cfg.TokenSecret, 24*time.Hour),
		User:  userResponse{ID: user.ID, Email: user.Email},
	}
}

type uploadInitRequest struct {
	ParentID          *string `json:"parentId"`
	EncryptedMetadata string  `json:"encryptedMetadata"`
	CiphertextSize    int64   `json:"ciphertextSize"`
}

type uploadInitResponse struct {
	UploadID  string    `json:"uploadId"`
	ObjectKey string    `json:"objectKey"`
	UploadURL string    `json:"uploadUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Server) initUpload(w http.ResponseWriter, r *http.Request, user store.User) {
	var req uploadInitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.EncryptedMetadata == "" || req.CiphertextSize < 0 {
		writeError(w, http.StatusBadRequest, "encrypted metadata and non-negative ciphertext size are required")
		return
	}
	objectKey := user.StoragePrefix + "/objects/" + store.NewID("obj")
	signed, err := s.objects.PresignPut(r.Context(), objectKey, req.CiphertextSize, s.cfg.UploadTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	upload, err := s.repo.CreateUpload(store.UploadSession{
		ID:                store.NewID("upload"),
		UserID:            user.ID,
		ParentID:          req.ParentID,
		EncryptedMetadata: req.EncryptedMetadata,
		CiphertextSize:    req.CiphertextSize,
		ObjectKey:         objectKey,
		ExpiresAt:         signed.ExpiresAt,
	})
	if err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, uploadInitResponse{
		UploadID:  upload.ID,
		ObjectKey: upload.ObjectKey,
		UploadURL: signed.URL,
		ExpiresAt: signed.ExpiresAt,
	})
}

func (s *Server) commitUpload(w http.ResponseWriter, r *http.Request, user store.User) {
	uploadID := r.PathValue("uploadId")
	upload, err := s.repo.UploadByID(user.ID, uploadID)
	if err != nil {
		statusForError(w, err)
		return
	}
	if time.Now().UTC().After(upload.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "upload session expired")
		return
	}
	file, err := s.repo.CommitUpload(user.ID, uploadID)
	if err != nil {
		statusForError(w, err)
		return
	}
	now := time.Now().UTC()
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: now})
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "storage_byte_hour", Quantity: file.CiphertextSize, CreatedAt: now})
	writeJSON(w, http.StatusCreated, file)
}

type createFolderRequest struct {
	ParentID          *string `json:"parentId"`
	EncryptedMetadata string  `json:"encryptedMetadata"`
	Name              string  `json:"name"`
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request, user store.User) {
	var req createFolderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" && req.EncryptedMetadata == "" {
		writeError(w, http.StatusBadRequest, "folder name is required")
		return
	}
	encryptedMetadata := req.EncryptedMetadata
	if encryptedMetadata == "" {
		var err error
		encryptedMetadata, err = s.encryptMetadata(user, fileMetadata{Name: strings.TrimSpace(req.Name)})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if req.ParentID != nil {
		if _, err := s.repo.FileByID(user.ID, *req.ParentID); err != nil {
			statusForError(w, err)
			return
		}
	}
	now := time.Now().UTC()
	file, err := s.repo.CreateFolder(store.FileRecord{
		ID:                store.NewID("file"),
		UserID:            user.ID,
		Kind:              store.FileKindFolder,
		ParentID:          req.ParentID,
		EncryptedMetadata: encryptedMetadata,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: now})
	s.writeFile(w, http.StatusCreated, user, file, false)
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request, user store.User) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart upload: "+err.Error())
		return
	}
	uploaded, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer uploaded.Close()
	plaintext, err := io.ReadAll(uploaded)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	parentID := stringPtrOrNil(r.FormValue("parentId"))
	if parentID != nil {
		if _, err := s.repo.FileByID(user.ID, *parentID); err != nil {
			statusForError(w, err)
			return
		}
	}
	name := r.FormValue("name")
	if strings.TrimSpace(name) == "" {
		name = header.Filename
	}
	mimeType := r.FormValue("mimeType")
	if mimeType == "" {
		mimeType = header.Header.Get("Content-Type")
	}
	lastModified, _ := strconv.ParseInt(r.FormValue("lastModified"), 10, 64)
	metadata := fileMetadata{Name: name, MimeType: mimeType, LastModified: lastModified}
	accountKey, err := s.accountKey(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ciphertext, nonce, err := cryptoutil.Encrypt(accountKey, plaintext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	encryptedMetadata, err := s.encryptMetadataWithKey(accountKey, metadata)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	objectKey := user.StoragePrefix + "/objects/" + store.NewID("obj")
	payload := cryptoutil.Pack(nonce, ciphertext)
	if err := s.objects.Put(r.Context(), objectKey, payload); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now().UTC()
	file, err := s.repo.CreateFile(store.FileRecord{
		ID:                store.NewID("file"),
		UserID:            user.ID,
		Kind:              store.FileKindFile,
		ParentID:          parentID,
		EncryptedMetadata: encryptedMetadata,
		CiphertextSize:    int64(len(payload)),
		ObjectKey:         &objectKey,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: now})
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "storage_byte_hour", Quantity: file.CiphertextSize, CreatedAt: now})
	s.writeFile(w, http.StatusCreated, user, file, false)
}

func (s *Server) listFiles(w http.ResponseWriter, _ *http.Request, user store.User) {
	files, err := s.repo.ListFiles(user.ID)
	if err != nil {
		statusForError(w, err)
		return
	}
	response := make([]store.FileRecord, 0, len(files))
	for _, file := range files {
		response = append(response, s.publicFile(user, file, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": response})
}

type updateFileRequest struct {
	ParentID          *string `json:"parentId"`
	EncryptedMetadata string  `json:"encryptedMetadata"`
	Name              string  `json:"name"`
}

func (s *Server) updateFile(w http.ResponseWriter, r *http.Request, user store.User) {
	var req updateFileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	encryptedMetadata := req.EncryptedMetadata
	if strings.TrimSpace(req.Name) != "" {
		var err error
		encryptedMetadata, err = s.encryptMetadata(user, fileMetadata{Name: strings.TrimSpace(req.Name)})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	file, err := s.repo.UpdateFile(user.ID, r.PathValue("fileId"), req.ParentID, encryptedMetadata)
	if err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: time.Now().UTC()})
	s.writeFile(w, http.StatusOK, user, file, false)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request, user store.User) {
	if err := s.repo.DeleteFile(user.ID, r.PathValue("fileId")); err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: time.Now().UTC()})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request, user store.User) {
	s.servePlaintextFile(w, r, user, r.PathValue("fileId"), false, true)
}

func (s *Server) previewFile(w http.ResponseWriter, r *http.Request, user store.User) {
	s.servePlaintextFile(w, r, user, r.PathValue("fileId"), false, false)
}

func (s *Server) servePlaintextFile(w http.ResponseWriter, r *http.Request, user store.User, fileID string, admin bool, attachment bool) {
	file, err := s.repo.FileByID(user.ID, fileID)
	if err != nil {
		statusForError(w, err)
		return
	}
	if admin {
		file, err = s.repo.AdminFileByID(fileID)
		if err != nil {
			statusForError(w, err)
			return
		}
		user, err = s.repo.UserByID(file.UserID)
		if err != nil {
			statusForError(w, err)
			return
		}
	}
	if file.Kind != store.FileKindFile || file.ObjectKey == nil {
		writeError(w, http.StatusBadRequest, "folders cannot be downloaded")
		return
	}
	plaintext, metadata, err := s.decryptFile(r.Context(), user, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now().UTC()
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: now})
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "egress_byte", Quantity: file.CiphertextSize, CreatedAt: now})
	contentType := metadata.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, url.PathEscape(metadata.Name)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(plaintext)
}

func (s *Server) adminDownloadFile(w http.ResponseWriter, r *http.Request) {
	s.serveAdminPlaintextFile(w, r, true)
}

func (s *Server) adminPreviewFile(w http.ResponseWriter, r *http.Request) {
	s.serveAdminPlaintextFile(w, r, false)
}

func (s *Server) serveAdminPlaintextFile(w http.ResponseWriter, r *http.Request, attachment bool) {
	file, err := s.repo.AdminFileByID(r.PathValue("fileId"))
	if err != nil {
		statusForError(w, err)
		return
	}
	user, err := s.repo.UserByID(file.UserID)
	if err != nil {
		statusForError(w, err)
		return
	}
	s.servePlaintextFile(w, r, user, file.ID, true, attachment)
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.repo.UsersByEmail(r.URL.Query().Get("email"))
	if err != nil {
		statusForError(w, err)
		return
	}
	type adminUser struct {
		ID            string    `json:"id"`
		Email         string    `json:"email"`
		StoragePrefix string    `json:"storagePrefix"`
		QuotaBytes    int64     `json:"quotaBytes"`
		UsedBytes     int64     `json:"usedBytes"`
		CreatedAt     time.Time `json:"createdAt"`
	}
	response := make([]adminUser, 0, len(users))
	for _, user := range users {
		response = append(response, adminUser{
			ID:            user.ID,
			Email:         user.Email,
			StoragePrefix: user.StoragePrefix,
			QuotaBytes:    user.QuotaBytes,
			UsedBytes:     user.UsedBytes,
			CreatedAt:     user.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": response})
}

func (s *Server) adminFiles(w http.ResponseWriter, r *http.Request) {
	user, err := s.repo.UserByID(r.PathValue("userId"))
	if err != nil {
		statusForError(w, err)
		return
	}
	files, err := s.repo.ListFiles(user.ID)
	if err != nil {
		statusForError(w, err)
		return
	}
	response := make([]store.FileRecord, 0, len(files))
	for _, file := range files {
		response = append(response, s.publicFile(user, file, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": response})
}

func (s *Server) usage(w http.ResponseWriter, _ *http.Request, user store.User) {
	events, err := s.repo.UsageForUser(user.ID)
	if err != nil {
		statusForError(w, err)
		return
	}
	cost, err := s.repo.ProviderCost("s3-compatible")
	if err != nil {
		statusForError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": billing.Summarize(events, cost),
		"cost":    cost,
	})
}

type authHandler func(http.ResponseWriter, *http.Request, store.User)
type adminHandler func(http.ResponseWriter, *http.Request)

func (s *Server) requireAuth(next authHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := strings.TrimPrefix(header, "Bearer ")
		if token == header || token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		userID, err := auth.VerifyToken(token, s.cfg.TokenSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		user, err := s.repo.UserByID(userID)
		if err != nil {
			statusForError(w, err)
			return
		}
		next(w, r, user)
	}
}

func (s *Server) requireAdmin(next adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := strings.TrimPrefix(header, "Bearer ")
		if token == header || token == "" || token != s.cfg.AdminToken {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}
		next(w, r)
	}
}

type fileMetadata struct {
	Name         string `json:"name"`
	MimeType     string `json:"mimeType,omitempty"`
	LastModified int64  `json:"lastModified,omitempty"`
}

func (s *Server) rootKey() []byte {
	return cryptoutil.RootKey(s.cfg.MasterKeyEncryptionSecret)
}

func (s *Server) accountKey(user store.User) ([]byte, error) {
	return cryptoutil.DecryptKey(s.rootKey(), cryptoutil.EncryptedKey{
		Ciphertext: user.EncryptedAccountMasterKey,
		Nonce:      user.AccountMasterKeyNonce,
	})
}

func (s *Server) encryptMetadata(user store.User, metadata fileMetadata) (string, error) {
	accountKey, err := s.accountKey(user)
	if err != nil {
		return "", err
	}
	return s.encryptMetadataWithKey(accountKey, metadata)
}

func (s *Server) encryptMetadataWithKey(accountKey []byte, metadata fileMetadata) (string, error) {
	plaintext, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	ciphertext, nonce, err := cryptoutil.Encrypt(accountKey, plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cryptoutil.Pack(nonce, ciphertext)), nil
}

func (s *Server) decryptMetadata(user store.User, encryptedMetadata string) (fileMetadata, error) {
	accountKey, err := s.accountKey(user)
	if err != nil {
		return fileMetadata{}, err
	}
	payload, err := base64.StdEncoding.DecodeString(encryptedMetadata)
	if err != nil {
		return fileMetadata{}, err
	}
	nonce, ciphertext, err := cryptoutil.Unpack(payload)
	if err != nil {
		return fileMetadata{}, err
	}
	plaintext, err := cryptoutil.Decrypt(accountKey, ciphertext, nonce)
	if err != nil {
		return fileMetadata{}, err
	}
	var metadata fileMetadata
	if err := json.Unmarshal(plaintext, &metadata); err != nil {
		return fileMetadata{}, err
	}
	return metadata, nil
}

func (s *Server) decryptFile(ctx context.Context, user store.User, file store.FileRecord) ([]byte, fileMetadata, error) {
	if file.ObjectKey == nil {
		return nil, fileMetadata{}, fmt.Errorf("missing object key")
	}
	accountKey, err := s.accountKey(user)
	if err != nil {
		return nil, fileMetadata{}, err
	}
	payload, err := s.objects.Get(ctx, *file.ObjectKey)
	if err != nil {
		return nil, fileMetadata{}, err
	}
	nonce, ciphertext, err := cryptoutil.Unpack(payload)
	if err != nil {
		return nil, fileMetadata{}, err
	}
	plaintext, err := cryptoutil.Decrypt(accountKey, ciphertext, nonce)
	if err != nil {
		return nil, fileMetadata{}, err
	}
	metadata, err := s.decryptMetadata(user, file.EncryptedMetadata)
	if err != nil {
		return nil, fileMetadata{}, err
	}
	return plaintext, metadata, nil
}

func (s *Server) publicFile(user store.User, file store.FileRecord, includeObjectKey bool) store.FileRecord {
	response := file
	if !includeObjectKey {
		response.ObjectKey = nil
	}
	metadata, err := s.decryptMetadata(user, file.EncryptedMetadata)
	if err == nil {
		response.Name = metadata.Name
		response.MimeType = metadata.MimeType
		response.LastModified = metadata.LastModified
	}
	return response
}

func (s *Server) writeFile(w http.ResponseWriter, status int, user store.User, file store.FileRecord, includeObjectKey bool) {
	writeJSON(w, status, s.publicFile(user, file, includeObjectKey))
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func statusForError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrQuotaExceeded):
		writeError(w, http.StatusPaymentRequired, err.Error())
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "authorization, content-type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
