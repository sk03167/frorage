package objectstore

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestS3PresignerCreatesPathStyleURL(t *testing.T) {
	store := NewS3Presigner(S3Config{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "frorage",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		PathStyle:       true,
	})
	signed, err := store.PresignPut(context.Background(), "users/u/objects/o", 10, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/frorage/users/u/objects/o" {
		t.Fatalf("unexpected path %q", parsed.Path)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("missing signature")
	}
}
