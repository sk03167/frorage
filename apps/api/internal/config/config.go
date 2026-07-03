package config

import (
	"os"
	"strconv"
	"time"

	"frorage/apps/api/internal/objectstore"
)

type Config struct {
	HTTPAddr                  string
	TokenSecret               string
	AdminToken                string
	AdminWebDist              string
	MasterKeyEncryptionSecret string
	DefaultQuotaGB            int64
	UploadTTL                 time.Duration
	DownloadTTL               time.Duration
	S3                        objectstore.S3Config
}

func FromEnv() Config {
	return Config{
		HTTPAddr:                  env("HTTP_ADDR", ":8080"),
		TokenSecret:               env("TOKEN_SECRET", "dev-token-secret-change-me"),
		AdminToken:                env("ADMIN_TOKEN", "dev-admin-token-change-me"),
		AdminWebDist:              env("ADMIN_WEB_DIST", "../../apps/web/dist"),
		MasterKeyEncryptionSecret: env("MASTER_KEY_ENCRYPTION_SECRET", "dev-master-key-encryption-secret-change-me"),
		DefaultQuotaGB:            int64(envInt("DEFAULT_QUOTA_GB", 10)),
		UploadTTL:                 time.Duration(envInt("UPLOAD_TTL_MINUTES", 15)) * time.Minute,
		DownloadTTL:               time.Duration(envInt("DOWNLOAD_TTL_MINUTES", 10)) * time.Minute,
		S3: objectstore.S3Config{
			Endpoint:        env("S3_ENDPOINT", "http://localhost:9000"),
			Region:          env("S3_REGION", "us-east-1"),
			Bucket:          env("S3_BUCKET", "frorage"),
			AccessKeyID:     env("S3_ACCESS_KEY_ID", "minioadmin"),
			SecretAccessKey: env("S3_SECRET_ACCESS_KEY", "minioadmin"),
			PathStyle:       envBool("S3_PATH_STYLE", true),
		},
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
