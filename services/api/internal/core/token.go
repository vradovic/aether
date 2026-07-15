package core

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type IssuedToken struct {
	Value            string
	ExpiresInSeconds int64
}

type TokenIssuer interface {
	Issue(userID string) (IssuedToken, error)
}

type AccessTokenIssuer struct {
	signingKey []byte
	issuer     string
	lifetime   time.Duration
	now        func() time.Time
}

func NewAccessTokenIssuer(signingKey, issuer string, lifetime time.Duration) *AccessTokenIssuer {
	return &AccessTokenIssuer{
		signingKey: []byte(signingKey),
		issuer:     issuer,
		lifetime:   lifetime,
		now:        time.Now,
	}
}

func (i *AccessTokenIssuer) Issue(userID string) (IssuedToken, error) {
	now := i.now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    i.issuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(i.lifetime)),
	}

	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.signingKey)
	if err != nil {
		return IssuedToken{}, err
	}

	return IssuedToken{
		Value:            value,
		ExpiresInSeconds: int64(i.lifetime / time.Second),
	}, nil
}
