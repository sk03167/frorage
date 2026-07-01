package store

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrQuotaExceeded = errors.New("quota exceeded")
)

type Repository interface {
	CreateUser(user User) (User, error)
	UserByEmail(email string) (User, error)
	UserByID(id string) (User, error)
	UsersByEmail(email string) ([]User, error)
	UpdatePasswordVerifier(userID string, passwordVerifier string) (User, error)
	CreateUpload(session UploadSession) (UploadSession, error)
	UploadByID(userID, uploadID string) (UploadSession, error)
	CommitUpload(userID, uploadID string) (FileRecord, error)
	CreateFile(file FileRecord) (FileRecord, error)
	CreateFolder(file FileRecord) (FileRecord, error)
	ListFiles(userID string) ([]FileRecord, error)
	FileByID(userID, fileID string) (FileRecord, error)
	AdminFileByID(fileID string) (FileRecord, error)
	UpdateFile(userID, fileID string, parentID *string, encryptedMetadata string) (FileRecord, error)
	DeleteFile(userID, fileID string) error
	CreatePasswordReset(reset PasswordReset) (PasswordReset, error)
	PasswordResetByToken(token string) (PasswordReset, error)
	MarkPasswordResetUsed(token string) error
	AddUsage(event UsageEvent) error
	UsageForUser(userID string) ([]UsageEvent, error)
	ProviderCost(provider string) (ProviderCost, error)
}

type MemoryRepository struct {
	mu            sync.RWMutex
	usersByID     map[string]User
	userIDByEmail map[string]string
	filesByID     map[string]FileRecord
	uploadsByID   map[string]UploadSession
	resetsByToken map[string]PasswordReset
	usage         []UsageEvent
	providerCosts map[string]ProviderCost
}

func NewMemoryRepository() *MemoryRepository {
	now := time.Now().UTC()
	return &MemoryRepository{
		usersByID:     map[string]User{},
		userIDByEmail: map[string]string{},
		filesByID:     map[string]FileRecord{},
		uploadsByID:   map[string]UploadSession{},
		resetsByToken: map[string]PasswordReset{},
		providerCosts: map[string]ProviderCost{
			"s3-compatible": {
				Provider:       "s3-compatible",
				StorageGBMonth: 18000,
				EgressGB:       90000,
				Operation10K:   4000,
				MarginBps:      300,
				UpdatedAt:      now,
			},
		},
	}
}

func (r *MemoryRepository) CreateUser(user User) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	email := normalizeEmail(user.Email)
	if _, ok := r.userIDByEmail[email]; ok {
		return User{}, ErrAlreadyExists
	}
	user.Email = email
	r.usersByID[user.ID] = user
	r.userIDByEmail[email] = user.ID
	return user, nil
}

func (r *MemoryRepository) UserByEmail(email string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.userIDByEmail[normalizeEmail(email)]
	if !ok {
		return User{}, ErrNotFound
	}
	return r.usersByID[id], nil
}

func (r *MemoryRepository) UserByID(id string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.usersByID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (r *MemoryRepository) UsersByEmail(email string) ([]User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := normalizeEmail(email)
	users := make([]User, 0)
	for _, user := range r.usersByID {
		if normalized == "" || strings.Contains(user.Email, normalized) {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users, nil
}

func (r *MemoryRepository) UpdatePasswordVerifier(userID string, passwordVerifier string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.usersByID[userID]
	if !ok {
		return User{}, ErrNotFound
	}
	user.PasswordVerifier = passwordVerifier
	r.usersByID[userID] = user
	return user, nil
}

func (r *MemoryRepository) CreateUpload(session UploadSession) (UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.usersByID[session.UserID]
	if !ok {
		return UploadSession{}, ErrNotFound
	}
	if user.UsedBytes+session.CiphertextSize > user.QuotaBytes {
		return UploadSession{}, ErrQuotaExceeded
	}
	r.uploadsByID[session.ID] = session
	return session, nil
}

func (r *MemoryRepository) UploadByID(userID, uploadID string) (UploadSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	upload, ok := r.uploadsByID[uploadID]
	if !ok || upload.UserID != userID {
		return UploadSession{}, ErrNotFound
	}
	return upload, nil
}

func (r *MemoryRepository) CommitUpload(userID, uploadID string) (FileRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	upload, ok := r.uploadsByID[uploadID]
	if !ok || upload.UserID != userID {
		return FileRecord{}, ErrNotFound
	}
	if upload.Committed {
		return FileRecord{}, ErrAlreadyExists
	}
	user := r.usersByID[userID]
	if user.UsedBytes+upload.CiphertextSize > user.QuotaBytes {
		return FileRecord{}, ErrQuotaExceeded
	}
	now := time.Now().UTC()
	file := FileRecord{
		ID:                NewID("file"),
		UserID:            userID,
		Kind:              FileKindFile,
		ParentID:          upload.ParentID,
		EncryptedMetadata: upload.EncryptedMetadata,
		CiphertextSize:    upload.CiphertextSize,
		ObjectKey:         &upload.ObjectKey,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	upload.Committed = true
	r.uploadsByID[uploadID] = upload
	r.filesByID[file.ID] = file
	user.UsedBytes += upload.CiphertextSize
	r.usersByID[userID] = user
	return file, nil
}

func (r *MemoryRepository) CreateFile(file FileRecord) (FileRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.usersByID[file.UserID]
	if !ok {
		return FileRecord{}, ErrNotFound
	}
	if user.UsedBytes+file.CiphertextSize > user.QuotaBytes {
		return FileRecord{}, ErrQuotaExceeded
	}
	r.filesByID[file.ID] = file
	user.UsedBytes += file.CiphertextSize
	r.usersByID[file.UserID] = user
	return file, nil
}

func (r *MemoryRepository) CreateFolder(file FileRecord) (FileRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.usersByID[file.UserID]; !ok {
		return FileRecord{}, ErrNotFound
	}
	r.filesByID[file.ID] = file
	return file, nil
}

func (r *MemoryRepository) ListFiles(userID string) ([]FileRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	files := make([]FileRecord, 0)
	for _, file := range r.filesByID {
		if file.UserID == userID {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedAt.Before(files[j].CreatedAt)
	})
	return files, nil
}

func (r *MemoryRepository) FileByID(userID, fileID string) (FileRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	file, ok := r.filesByID[fileID]
	if !ok || file.UserID != userID {
		return FileRecord{}, ErrNotFound
	}
	return file, nil
}

func (r *MemoryRepository) AdminFileByID(fileID string) (FileRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	file, ok := r.filesByID[fileID]
	if !ok {
		return FileRecord{}, ErrNotFound
	}
	return file, nil
}

func (r *MemoryRepository) UpdateFile(userID, fileID string, parentID *string, encryptedMetadata string) (FileRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	file, ok := r.filesByID[fileID]
	if !ok || file.UserID != userID {
		return FileRecord{}, ErrNotFound
	}
	file.ParentID = parentID
	if encryptedMetadata != "" {
		file.EncryptedMetadata = encryptedMetadata
	}
	file.UpdatedAt = time.Now().UTC()
	r.filesByID[fileID] = file
	return file, nil
}

func (r *MemoryRepository) DeleteFile(userID, fileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	file, ok := r.filesByID[fileID]
	if !ok || file.UserID != userID {
		return ErrNotFound
	}
	delete(r.filesByID, fileID)
	if file.Kind == FileKindFile {
		user := r.usersByID[userID]
		user.UsedBytes -= file.CiphertextSize
		if user.UsedBytes < 0 {
			user.UsedBytes = 0
		}
		r.usersByID[userID] = user
	}
	return nil
}

func (r *MemoryRepository) CreatePasswordReset(reset PasswordReset) (PasswordReset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.usersByID[reset.UserID]; !ok {
		return PasswordReset{}, ErrNotFound
	}
	r.resetsByToken[reset.Token] = reset
	return reset, nil
}

func (r *MemoryRepository) PasswordResetByToken(token string) (PasswordReset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reset, ok := r.resetsByToken[token]
	if !ok {
		return PasswordReset{}, ErrNotFound
	}
	return reset, nil
}

func (r *MemoryRepository) MarkPasswordResetUsed(token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	reset, ok := r.resetsByToken[token]
	if !ok {
		return ErrNotFound
	}
	reset.Used = true
	r.resetsByToken[token] = reset
	return nil
}

func (r *MemoryRepository) AddUsage(event UsageEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage = append(r.usage, event)
	return nil
}

func (r *MemoryRepository) UsageForUser(userID string) ([]UsageEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := make([]UsageEvent, 0)
	for _, event := range r.usage {
		if event.UserID == userID {
			events = append(events, event)
		}
	}
	return events, nil
}

func (r *MemoryRepository) ProviderCost(provider string) (ProviderCost, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cost, ok := r.providerCosts[provider]
	if !ok {
		return ProviderCost{}, ErrNotFound
	}
	return cost, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
