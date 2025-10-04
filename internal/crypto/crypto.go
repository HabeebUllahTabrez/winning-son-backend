package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// EncryptionService handles encryption/decryption and blind indexing
type EncryptionService struct {
	encryptionKey []byte // 32 bytes for AES-256
	blindIndexKey []byte // Separate key for HMAC blind indexing
}

// NewEncryptionService creates a new encryption service
// encryptionKey should be 32 bytes for AES-256
// blindIndexKey should be 32 bytes for HMAC-SHA256
func NewEncryptionService(encryptionKey, blindIndexKey []byte) (*EncryptionService, error) {
	if len(encryptionKey) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	if len(blindIndexKey) != 32 {
		return nil, errors.New("blind index key must be 32 bytes")
	}
	return &EncryptionService{
		encryptionKey: encryptionKey,
		blindIndexKey: blindIndexKey,
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64-encoded ciphertext with nonce prepended
func (s *EncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM
func (s *EncryptionService) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, cipherBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GenerateBlindIndex creates a deterministic hash for searching encrypted data
// Uses HMAC-SHA256 to create a searchable index without revealing the plaintext
func (s *EncryptionService) GenerateBlindIndex(plaintext string) string {
	if plaintext == "" {
		return ""
	}

	h := hmac.New(sha256.New, s.blindIndexKey)
	h.Write([]byte(plaintext))
	hash := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(hash)
}

// EncryptWithBlindIndex encrypts data and returns both encrypted value and blind index
func (s *EncryptionService) EncryptWithBlindIndex(plaintext string) (encrypted, blindIndex string, err error) {
	encrypted, err = s.Encrypt(plaintext)
	if err != nil {
		return "", "", err
	}
	blindIndex = s.GenerateBlindIndex(plaintext)
	return encrypted, blindIndex, nil
}
