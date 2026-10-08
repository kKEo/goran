// Package util holds the small cryptographic helpers the server relies on:
// random tokens, token hashing and the AES-GCM box used for secrets at rest.
package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewToken returns prefix + 48 hex characters of randomness, for example
// "gu_3f9a...". Prefixes make it obvious which kind of credential leaked:
// gu_ user token, ga_ agent key, gr_ registration token.
func NewToken(prefix string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

// Hash is the value stored in the database for any token.
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Prefix is the short, non-secret part shown in listings.
func Prefix(token string) string {
	if len(token) > 10 {
		return token[:10]
	}
	return token
}
