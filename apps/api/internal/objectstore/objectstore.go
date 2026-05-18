package objectstore

import (
	"context"
	"time"
)

type SignedURL struct {
	URL       string
	ExpiresAt time.Time
}

type ObjectStore interface {
	PresignPut(ctx context.Context, objectKey string, size int64, ttl time.Duration) (SignedURL, error)
	PresignGet(ctx context.Context, objectKey string, ttl time.Duration) (SignedURL, error)
}
