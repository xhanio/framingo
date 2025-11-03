package ioutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"os"

	"github.com/xhanio/errors"
)

func EncryptAES(data []byte, password string) ([]byte, error) {

	// Hash the password to create a 32-byte key
	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new AES cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new GCM cipher")
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

func DecryptAES(data []byte, password string) ([]byte, error) {

	// Hash the password to create a 32-byte key
	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new AES cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new GCM cipher")
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.Newf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decrypt ciphertext")
	}
	return plaintext, nil

}

func EncryptAESFile(src, dst string, password string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return errors.Wrap(err)
	}
	encrypted, err := EncryptAES(b, password)
	if err != nil {
		return errors.Wrap(err)
	}
	err = os.WriteFile(dst, encrypted, 0644)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func DecryptAESFile(src, dst string, password string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return errors.Wrap(err)
	}
	decrpyted, err := DecryptAES(b, password)
	if err != nil {
		return errors.Wrap(err)
	}
	err = os.WriteFile(dst, decrpyted, 0644)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}
