package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	UserID uint `json:"uid"`
	jwt.RegisteredClaims
}

func CreateUserToken(secret string, userID uint, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", fmt.Errorf("invalid token ttl")
	}

	now := time.Now()
	claims := UserClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("user:%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func ParseUserToken(secret, tokenStr string) (*UserClaims, error) {
	parsed, err := jwt.ParseWithClaims(
		tokenStr,
		&UserClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*UserClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
