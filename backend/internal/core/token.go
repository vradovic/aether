package core

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidAuthorization = errors.New("invalid authorization header")

func IssueToken(signingKey, userID string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject: userID,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(signingKey))
	if err != nil {
		return "", err
	}

	return token, nil
}

func ExtractBearerToken(header string) (string, error) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok ||
		!strings.EqualFold(scheme, "Bearer") ||
		strings.TrimSpace(token) == "" ||
		strings.ContainsAny(token, " \t\r\n") {
		return "", ErrInvalidAuthorization
	}

	return token, nil
}

func ParseTokenClaims(tokenString, signingKey string) (jwt.Claims, error) {
	claims := &jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			return []byte(signingKey), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	return claims, err
}

func ParseTokenSubject(tokenString, signingKey string) (string, error) {
	claims, err := ParseTokenClaims(tokenString, signingKey)
	if err != nil {
		return "", err
	}

	sub, err := claims.GetSubject()
	if err != nil {
		return "", err
	}

	return sub, nil
}
