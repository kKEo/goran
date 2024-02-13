package util

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

type Encoding int

const (
	Hex Encoding = iota
	Base64
)

func (e Encoding) String() string {
	return [...]string{"Hex", "Base64"}[e]
}

func GenerateToken(tokenLength int, encodingType Encoding) (string, error) {
	b := make([]byte, tokenLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	if encodingType == Hex {
		return hex.EncodeToString(b), nil
	} else {
		return base64.URLEncoding.EncodeToString(b), nil
	}
}
