package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func GenerateAgentToken() (rawToken string, tokenHash string, tokenPrefix string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", err
	}

	rawToken = base64.RawURLEncoding.EncodeToString(buf)
	tokenHash = HashAgentToken(rawToken)
	if len(rawToken) > 12 {
		tokenPrefix = rawToken[:12]
	} else {
		tokenPrefix = rawToken
	}

	return rawToken, tokenHash, tokenPrefix, nil
}

func HashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
