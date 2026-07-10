package auth

import (
	"fmt"
	"net/mail"
)

const minPasswordLength = 8
const maxPasswordLength = 50
const minNameLength = 2
const maxNameLength = 50

var errPasswordLength = fmt.Errorf("password should be between %d and %d characters long", minPasswordLength, maxPasswordLength)
var errNameLength = fmt.Errorf("name should be between %d and %d characters long", minNameLength, maxNameLength)
var errEmailFormat = fmt.Errorf("invalid email format")

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func (r registerRequest) validate() error {
	if len(r.FirstName) < minNameLength ||
		len(r.FirstName) > maxNameLength ||
		len(r.LastName) < minNameLength ||
		len(r.LastName) > maxNameLength {
		return errNameLength
	}

	addr, err := mail.ParseAddress(r.Email)
	if err != nil || addr.Address != r.Email { // Required to check Address because ParseAddress parses "Name <name@email.com>" as well
		return errEmailFormat
	}

	if len(r.Password) < minPasswordLength ||
		len(r.Password) > maxPasswordLength {
		return errPasswordLength
	}

	return nil
}
