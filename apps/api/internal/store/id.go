package store

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
)

func NewID(prefix string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return prefix + "_" + strings.TrimRight(token, "=")
}
