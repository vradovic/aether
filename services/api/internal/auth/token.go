package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type issuedToken struct {
	value            string
	expiresInSeconds int64
}

type tokenIssuer interface {
	issue(userID string) (issuedToken, error)
}

type accessTokenIssuer struct {
	signingKey []byte
	issuer     string
	lifetime   time.Duration
	now        func() time.Time
}

func NewAccessTokenIssuer(signingKey, issuer string, lifetime time.Duration) *accessTokenIssuer {
	return &accessTokenIssuer{
		signingKey: []byte(signingKey),
		issuer:     issuer,
		lifetime:   lifetime,
		now:        time.Now,
	}
}

func (i *accessTokenIssuer) issue(userID string) (issuedToken, error) {
	now := i.now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    i.issuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(i.lifetime)),
	}

	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.signingKey)
	if err != nil {
		return issuedToken{}, err
	}

	return issuedToken{
		value:            value,
		expiresInSeconds: int64(i.lifetime / time.Second),
	}, nil
}
