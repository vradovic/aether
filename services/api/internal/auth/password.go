package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var errInvalidCredentials = errors.New("invalid credentials")

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcryptCost,
	)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func verifyPassword(password, passwordHash string) error {
	err := bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(password),
	)

	if err != nil {
		return errInvalidCredentials
	}

	return nil
}
