package user

import (
	"github.com/xhanio/errors"
	"golang.org/x/crypto/bcrypt"
)

func (m *manager) hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), m.passwordCost)
	if err != nil {
		// bcrypt refuses a password over 72 bytes
		return "", errors.InvalidArgument.Wrap(err)
	}
	return string(h), nil
}

// dummyHash makes a failed lookup cost a bcrypt compare so it cannot enumerate
// usernames; derived once, at the manager's cost, so it costs what a real one does.
func (m *manager) dummyHash() string {
	return m.dummy()
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
