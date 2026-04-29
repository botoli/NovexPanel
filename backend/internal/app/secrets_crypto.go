package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func deriveMasterKey(secret string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return sum[:]
}

func encryptSecretEnvelope(masterKey []byte, plain string) (encryptedValue, encryptedDEK, nonceB64 string, err error) {
	dek := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, dek); err != nil {
		return "", "", "", err
	}
	nonce := make([]byte, 12)
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", "", err
	}
	encryptedValue, err = encryptWithKey(dek, nonce, []byte(plain))
	if err != nil {
		return "", "", "", err
	}
	encryptedDEK, err = encryptWithKey(masterKey, nonce, dek)
	if err != nil {
		return "", "", "", err
	}
	return encryptedValue, encryptedDEK, base64.StdEncoding.EncodeToString(nonce), nil
}

func decryptSecretEnvelope(masterKey []byte, encryptedValue, encryptedDEK, nonceB64 string) (string, error) {
	nonce, err := base64.StdEncoding.DecodeString(strings.TrimSpace(nonceB64))
	if err != nil {
		return "", err
	}
	dek, err := decryptWithKey(masterKey, nonce, encryptedDEK)
	if err != nil {
		return "", err
	}
	plain, err := decryptWithKey(dek, nonce, encryptedValue)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func encryptWithKey(key, nonce, plain []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nil, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func decryptWithKey(key, nonce []byte, cipherB64 string) ([]byte, error) {
	cipherRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cipherB64))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, cipherRaw, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed")
	}
	return plain, nil
}

func maskSecret(value string) string {
	v := strings.TrimSpace(value)
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + strings.Repeat("*", len(v)-4) + v[len(v)-2:]
}
