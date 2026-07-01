package store

import "time"

type User struct {
	ID                        string    `json:"id"`
	Email                     string    `json:"email"`
	PasswordVerifier          string    `json:"-"`
	StoragePrefix             string    `json:"storagePrefix"`
	EncryptedAccountMasterKey string    `json:"-"`
	AccountMasterKeyNonce     string    `json:"-"`
	QuotaBytes                int64     `json:"quotaBytes"`
	UsedBytes                 int64     `json:"usedBytes"`
	CreatedAt                 time.Time `json:"createdAt"`
}

type FileKind string

const (
	FileKindFile   FileKind = "file"
	FileKindFolder FileKind = "folder"
)

type FileRecord struct {
	ID                string    `json:"id"`
	UserID            string    `json:"-"`
	Kind              FileKind  `json:"kind"`
	ParentID          *string   `json:"parentId"`
	EncryptedMetadata string    `json:"encryptedMetadata"`
	Name              string    `json:"name,omitempty"`
	MimeType          string    `json:"mimeType,omitempty"`
	LastModified      int64     `json:"lastModified,omitempty"`
	CiphertextSize    int64     `json:"ciphertextSize"`
	ObjectKey         *string   `json:"objectKey"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type UploadSession struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	ParentID          *string   `json:"parentId"`
	EncryptedMetadata string    `json:"encryptedMetadata"`
	CiphertextSize    int64     `json:"ciphertextSize"`
	ObjectKey         string    `json:"objectKey"`
	ExpiresAt         time.Time `json:"expiresAt"`
	Committed         bool      `json:"committed"`
}

type PasswordReset struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Used      bool      `json:"used"`
}

type UsageEvent struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Metric    string    `json:"metric"`
	Quantity  int64     `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProviderCost struct {
	Provider       string    `json:"provider"`
	StorageGBMonth int64     `json:"storageGbMonthMicros"`
	EgressGB       int64     `json:"egressGbMicros"`
	Operation10K   int64     `json:"operation10kMicros"`
	MarginBps      int64     `json:"marginBps"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
