package user

import (
	"sync"

	"github.com/xhanio/errors"
	"golang.org/x/crypto/bcrypt"
)

const passwordCost = bcrypt.DefaultCost

// dummyHash makes a failed lookup cost a bcrypt compare so it cannot enumerate
// usernames; derived at passwordCost so it tracks cost changes.
var dummyHash = sync.OnceValue(func() string {
	h, err := hashPassword("dummy")
	if err != nil {
		panic(err)
	}
	return h
})

func hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	if err != nil {
		// bcrypt refuses a password over 72 bytes
		return "", errors.InvalidArgument.Wrap(err)
	}
	return string(h), nil
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
