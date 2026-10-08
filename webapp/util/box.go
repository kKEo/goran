package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// KeySize is the master key length in bytes (AES-256).
const KeySize = 32

// NewKey generates a random master key.
func NewKey() ([]byte, error) {
	k := make([]byte, KeySize)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	return k, nil
}

// EncodeKey renders a key the way GORAN_MASTER_KEY expects it.
func EncodeKey(k []byte) string { return hex.EncodeToString(k) }

// ParseKey reads a hex encoded 32-byte key.
func ParseKey(s string) ([]byte, error) {
	k, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("master key must be hex: %w", err)
	}
	if len(k) != KeySize {
		return nil, fmt.Errorf("master key must be %d bytes (%d hex characters), got %d bytes", KeySize, KeySize*2, len(k))
	}
	return k, nil
}

// Box seals and opens secrets with AES-256-GCM. Each ciphertext is
// nonce || sealed bytes.
type Box struct {
	aead cipher.AEAD
}

func NewBox(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("master key must be %d bytes", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, b.aead.Seal(nil, nonce, plaintext, nil)...), nil
}

func (b *Box) Open(data []byte) ([]byte, error) {
	ns := b.aead.NonceSize()
	if len(data) < ns {
		return nil, errors.New("ciphertext too short")
	}
	return b.aead.Open(nil, data[:ns], data[ns:], nil)
}
