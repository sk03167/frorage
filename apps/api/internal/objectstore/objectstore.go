package objectstore

import (
	"context"
	"errors"
	"time"
)

var ErrObjectNotFound = errors.New("object not found")

type SignedURL struct {
	URL       string
	ExpiresAt time.Time
}

type ObjectStore interface {
	PresignPut(ctx context.Context, objectKey string, size int64, ttl time.Duration) (SignedURL, error)
	PresignGet(ctx context.Context, objectKey string, ttl time.Duration) (SignedURL, error)
	Put(ctx context.Context, objectKey string, data []byte) error
	Get(ctx context.Context, objectKey string) ([]byte, error)
}
