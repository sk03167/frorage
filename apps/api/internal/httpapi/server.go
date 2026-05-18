package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"private-cloud-storage/apps/api/internal/auth"
	"private-cloud-storage/apps/api/internal/billing"
	"private-cloud-storage/apps/api/internal/config"
	"private-cloud-storage/apps/api/internal/objectstore"
	"private-cloud-storage/apps/api/internal/store"
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
	s.mux.HandleFunc("POST /v1/auth/recover", s.recover)
	s.mux.HandleFunc("POST /v1/uploads/init", s.requireAuth(s.initUpload))
	s.mux.HandleFunc("POST /v1/uploads/{uploadId}/commit", s.requireAuth(s.commitUpload))
	s.mux.HandleFunc("GET /v1/files", s.requireAuth(s.listFiles))
	s.mux.HandleFunc("POST /v1/files", s.requireAuth(s.createFolder))
	s.mux.HandleFunc("PATCH /v1/files/{fileId}", s.requireAuth(s.updateFile))
	s.mux.HandleFunc("DELETE /v1/files/{fileId}", s.requireAuth(s.deleteFile))
	s.mux.HandleFunc("POST /v1/files/{fileId}/download", s.requireAuth(s.downloadFile))
	s.mux.HandleFunc("GET /v1/billing/usage", s.requireAuth(s.usage))
}

type signupRequest struct {
	Email            string          `json:"email"`
	PasswordVerifier string          `json:"passwordVerifier"`
	KeyBundle        store.KeyBundle `json:"keyBundle"`
}

type loginRequest struct {
	Email            string `json:"email"`
	PasswordVerifier string `json:"passwordVerifier"`
}

type authResponse struct {
	Token     string          `json:"token"`
	User      userResponse    `json:"user"`
	KeyBundle store.KeyBundle `json:"keyBundle"`
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
	if req.Email == "" || req.PasswordVerifier == "" || req.KeyBundle.PasswordWrappedMasterKey == "" {
		writeError(w, http.StatusBadRequest, "email, password verifier, and key bundle are required")
		return
	}
	now := time.Now().UTC()
	user, err := s.repo.CreateUser(store.User{
		ID:               store.NewID("user"),
		Email:            req.Email,
		PasswordVerifier: req.PasswordVerifier,
		KeyBundle:        req.KeyBundle,
		QuotaBytes:       s.cfg.DefaultQuotaGB * 1024 * 1024 * 1024,
		CreatedAt:        now,
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

type recoverRequest struct {
	Email            string          `json:"email"`
	PasswordVerifier string          `json:"passwordVerifier"`
	KeyBundle        store.KeyBundle `json:"keyBundle"`
}

func (s *Server) recover(w http.ResponseWriter, r *http.Request) {
	var req recoverRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.repo.UserByEmail(req.Email)
	if err != nil {
		statusForError(w, err)
		return
	}
	updated, err := s.repo.UpdateUserKeyBundle(user.ID, req.KeyBundle, req.PasswordVerifier)
	if err != nil {
		statusForError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.authResponse(updated))
}

func (s *Server) authResponse(user store.User) authResponse {
	return authResponse{
		Token:     auth.SignToken(user.ID, s.cfg.TokenSecret, 24*time.Hour),
		User:      userResponse{ID: user.ID, Email: user.Email},
		KeyBundle: user.KeyBundle,
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
	objectKey := "users/" + user.ID + "/objects/" + store.NewID("obj")
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
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request, user store.User) {
	var req createFolderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.EncryptedMetadata == "" {
		writeError(w, http.StatusBadRequest, "encrypted metadata is required")
		return
	}
	now := time.Now().UTC()
	file, err := s.repo.CreateFolder(store.FileRecord{
		ID:                store.NewID("file"),
		UserID:            user.ID,
		Kind:              store.FileKindFolder,
		ParentID:          req.ParentID,
		EncryptedMetadata: req.EncryptedMetadata,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: now})
	writeJSON(w, http.StatusCreated, file)
}

func (s *Server) listFiles(w http.ResponseWriter, _ *http.Request, user store.User) {
	files, err := s.repo.ListFiles(user.ID)
	if err != nil {
		statusForError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

type updateFileRequest struct {
	ParentID          *string `json:"parentId"`
	EncryptedMetadata string  `json:"encryptedMetadata"`
}

func (s *Server) updateFile(w http.ResponseWriter, r *http.Request, user store.User) {
	var req updateFileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	file, err := s.repo.UpdateFile(user.ID, r.PathValue("fileId"), req.ParentID, req.EncryptedMetadata)
	if err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request, user store.User) {
	if err := s.repo.DeleteFile(user.ID, r.PathValue("fileId")); err != nil {
		statusForError(w, err)
		return
	}
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: time.Now().UTC()})
	w.WriteHeader(http.StatusNoContent)
}

type downloadResponse struct {
	DownloadURL string    `json:"downloadUrl"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request, user store.User) {
	file, err := s.repo.FileByID(user.ID, r.PathValue("fileId"))
	if err != nil {
		statusForError(w, err)
		return
	}
	if file.Kind != store.FileKindFile || file.ObjectKey == nil {
		writeError(w, http.StatusBadRequest, "folders cannot be downloaded")
		return
	}
	signed, err := s.objects.PresignGet(r.Context(), *file.ObjectKey, s.cfg.DownloadTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now().UTC()
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "object_operation", Quantity: 1, CreatedAt: now})
	_ = s.repo.AddUsage(store.UsageEvent{ID: store.NewID("usage"), UserID: user.ID, Metric: "egress_byte", Quantity: file.CiphertextSize, CreatedAt: now})
	writeJSON(w, http.StatusOK, downloadResponse{DownloadURL: signed.URL, ExpiresAt: signed.ExpiresAt})
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
