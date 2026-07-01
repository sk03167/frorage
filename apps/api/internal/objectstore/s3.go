package objectstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PathStyle       bool
}

type S3Presigner struct {
	cfg S3Config
}

func NewS3Presigner(cfg S3Config) *S3Presigner {
	return &S3Presigner{cfg: cfg}
}

func (s *S3Presigner) PresignPut(_ context.Context, objectKey string, _ int64, ttl time.Duration) (SignedURL, error) {
	return s.presign("PUT", objectKey, ttl)
}

func (s *S3Presigner) PresignGet(_ context.Context, objectKey string, ttl time.Duration) (SignedURL, error) {
	return s.presign("GET", objectKey, ttl)
}

func (s *S3Presigner) Put(ctx context.Context, objectKey string, data []byte) error {
	signed, err := s.PresignPut(ctx, objectKey, int64(len(data)), 15*time.Minute)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, signed.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("object put failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *S3Presigner) Get(ctx context.Context, objectKey string) ([]byte, error) {
	signed, err := s.PresignGet(ctx, objectKey, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signed.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("object get failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func (s *S3Presigner) presign(method, objectKey string, ttl time.Duration) (SignedURL, error) {
	if s.cfg.Endpoint == "" || s.cfg.Bucket == "" || s.cfg.AccessKeyID == "" || s.cfg.SecretAccessKey == "" {
		return SignedURL{}, errors.New("incomplete s3 configuration")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	parsed, err := url.Parse(s.cfg.Endpoint)
	if err != nil {
		return SignedURL{}, err
	}

	bucketPath := "/" + path.Join(s.cfg.Bucket, objectKey)
	host := parsed.Host
	canonicalURI := bucketPath
	if !s.cfg.PathStyle {
		host = s.cfg.Bucket + "." + host
		canonicalURI = "/" + path.Clean(objectKey)
	}

	credentialDate := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	scope := credentialDate + "/" + s.cfg.Region + "/s3/aws4_request"
	values := url.Values{}
	values.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	values.Set("X-Amz-Credential", s.cfg.AccessKeyID+"/"+scope)
	values.Set("X-Amz-Date", amzDate)
	values.Set("X-Amz-Expires", fmt.Sprintf("%.0f", ttl.Seconds()))
	values.Set("X-Amz-SignedHeaders", "host")

	canonicalQuery := canonicalQueryString(values)
	canonicalHeaders := "host:" + strings.ToLower(host) + "\n"
	canonicalRequest := strings.Join([]string{
		method,
		encodePath(canonicalURI),
		canonicalQuery,
		canonicalHeaders,
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")

	hash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(hash[:]),
	}, "\n")
	signingKey := s.signingKey(credentialDate)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	values.Set("X-Amz-Signature", signature)

	presigned := *parsed
	presigned.Host = host
	presigned.Path = canonicalURI
	presigned.RawQuery = values.Encode()
	return SignedURL{URL: presigned.String(), ExpiresAt: expiresAt}, nil
}

func (s *S3Presigner) signingKey(date string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+s.cfg.SecretAccessKey), date)
	kRegion := hmacSHA256(kDate, s.cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func canonicalQueryString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		vals := values[key]
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func encodePath(value string) string {
	escaped := (&url.URL{Path: value}).EscapedPath()
	return strings.ReplaceAll(escaped, "%2F", "/")
}
