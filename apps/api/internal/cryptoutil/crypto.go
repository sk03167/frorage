package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const KeyBytes = 32

type EncryptedKey struct {
	Ciphertext string
	Nonce      string
}

func NewAccountKey() ([]byte, error) {
	return randomBytes(KeyBytes)
}

func RootKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func EncryptKey(rootKey, accountKey []byte) (EncryptedKey, error) {
	ciphertext, nonce, err := Encrypt(rootKey, accountKey)
	if err != nil {
		return EncryptedKey{}, err
	}
	return EncryptedKey{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
	}, nil
}

func DecryptKey(rootKey []byte, encrypted EncryptedKey) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return nil, err
	}
	return Decrypt(rootKey, ciphertext, nonce)
}

func Encrypt(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce, err := randomBytes(gcm.NonceSize())
	if err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func Decrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size")
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func Pack(nonce, ciphertext []byte) []byte {
	payload := make([]byte, 0, len(nonce)+len(ciphertext))
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)
	return payload
}

func Unpack(payload []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(make([]byte, KeyBytes))
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return nil, nil, fmt.Errorf("ciphertext too short")
	}
	return payload[:nonceSize], payload[nonceSize:], nil
}

func randomBytes(length int) ([]byte, error) {
	value := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return nil, err
	}
	return value, nil
}
