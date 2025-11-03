package ioutil

import (
	"context"
	"crypto/cipher"
	"crypto/des"
	"io"

	"github.com/xhanio/errors"
)

var DefaultDESDecryptorBufSize = 32 * 1024

type desDecryptor struct {
	ctx  context.Context
	size int64
	curr int64
	m    cipher.BlockMode
	r    io.Reader
}

func (r *desDecryptor) Read(buf []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		return 0, errors.Cancaled
	default:
		block := make([]byte, r.m.BlockSize())
		n, err = io.ReadFull(r.r, block)
		if err != nil {
			return n, errors.Wrap(err)
		}
		read := r.curr + int64(n)
		decrypted := make([]byte, n)
		// fmt.Println("n:", n, "block size:", r.m.BlockSize())
		r.m.CryptBlocks(decrypted[:n], block[:n])
		if r.size == read {
			// fmt.Printf("buf is [%d] %v\n\n", len(buf), buf)
			// fmt.Printf("dec is [%d] %v\n\n", len(decrypted), decrypted)
			padding := int(decrypted[n-1]) // last byte stands for the padding of src
			unpadding := make([]byte, n)
			copy(unpadding, decrypted[:n-padding])
			decrypted = unpadding
			// fmt.Printf("mtu is %d, read %d from src, buf size is %d, padding is %d\n\n", mtu, n, len(buf), padding)
			// fmt.Printf("dec is [%d] %v\n\n", len(decrypted), decrypted)
		}
		copy(buf, decrypted)
		// if r.size == read {
		// 	fmt.Printf("buf is [%d] %v\n\n", len(buf), buf)
		// }
		r.curr = read
		return n, nil
	}
}

func DESDecryptor(ctx context.Context, key, iv []byte, cipherReader io.Reader, size int64) (io.Reader, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	br := &desDecryptor{
		ctx:  ctx,
		size: size,
		m:    cipher.NewCBCDecrypter(block, iv),
		r:    cipherReader,
	}
	return br, nil
}
