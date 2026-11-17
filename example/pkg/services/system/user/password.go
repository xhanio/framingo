package user

import (
	"sync"

	"github.com/xhanio/errors"
	"golang.org/x/crypto/bcrypt"
)

// The password policy lives in this one file: how a password is hashed for
// storage, how a login is checked against the stored hash, and the decoy that
// keeps a failed lookup from answering faster than a wrong password. The
// repository only stores and returns the hash.

// passwordCost is the bcrypt cost every stored password is hashed at.
const passwordCost = bcrypt.DefaultCost

// dummyHash is compared against when the account lookup fails, so that path
// costs the same as a wrong-password attempt. Returning early instead makes
// "no such user" answer in ~1ms against bcrypt's ~80ms, which is a reliable
// oracle for enumerating valid usernames.
//
// It is derived at passwordCost rather than pasted as a literal: a hardcoded
// hash silently stops masking anything the day that cost changes. Computed
// once, on first use.
var dummyHash = sync.OnceValue(func() string {
	h, err := hashPassword("dummy")
	if err != nil {
		// Only possible on an invalid cost, which is a compile-time constant here.
		panic(err)
	}
	return h
})

func hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	if err != nil {
		// The cost is fixed, so what is left to fail on is the password
		// itself: bcrypt refuses one longer than 72 bytes.
		return "", errors.InvalidArgument.Wrap(err)
	}
	return string(h), nil
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
