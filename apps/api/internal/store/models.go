package store

import "time"

type User struct {
	ID               string    `json:"id"`
	Email            string    `json:"email"`
	PasswordVerifier string    `json:"-"`
	KeyBundle        KeyBundle `json:"keyBundle"`
	QuotaBytes       int64     `json:"quotaBytes"`
	UsedBytes        int64     `json:"usedBytes"`
	CreatedAt        time.Time `json:"createdAt"`
}

type KeyBundle struct {
	PasswordWrappedMasterKey       string `json:"passwordWrappedMasterKey"`
	PasswordKdfSalt                string `json:"passwordKdfSalt"`
	RecoveryPhraseWrappedMasterKey string `json:"recoveryPhraseWrappedMasterKey"`
	RecoveryFileWrappedMasterKey   string `json:"recoveryFileWrappedMasterKey"`
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
