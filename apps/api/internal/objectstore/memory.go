package objectstore

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: map[string][]byte{}}
}

func (s *MemoryStore) PresignPut(_ context.Context, objectKey string, _ int64, ttl time.Duration) (SignedURL, error) {
	return SignedURL{URL: "memory://" + objectKey, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *MemoryStore) PresignGet(_ context.Context, objectKey string, ttl time.Duration) (SignedURL, error) {
	return SignedURL{URL: "memory://" + objectKey, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *MemoryStore) Put(_ context.Context, objectKey string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copyData := make([]byte, len(data))
	copy(copyData, data)
	s.objects[objectKey] = copyData
	return nil
}

func (s *MemoryStore) Get(_ context.Context, objectKey string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.objects[objectKey]
	if !ok {
		return nil, ErrObjectNotFound
	}
	copyData := make([]byte, len(data))
	copy(copyData, data)
	return copyData, nil
}
