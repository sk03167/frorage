package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")

func SignToken(userID, secret string, ttl time.Duration) string {
	expires := time.Now().UTC().Add(ttl).Unix()
	payload := userID + "." + strconv.FormatInt(expires, 10)
	signature := sign(payload, secret)
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "." + signature))
}

func VerifyToken(token, secret string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", ErrInvalidToken
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 3 {
		return "", ErrInvalidToken
	}
	payload := parts[0] + "." + parts[1]
	expected := sign(payload, secret)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return "", ErrInvalidToken
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().UTC().Unix() > expires {
		return "", ErrInvalidToken
	}
	return parts[0], nil
}

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
