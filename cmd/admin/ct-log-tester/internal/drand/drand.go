// Package drand implements primitives for deterministic,
// cryptographically-secure randomness.
package drand

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"io"
)

type devZero struct{}

func (dz devZero) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = 0
	}
	return len(p), nil
}

type undoAGL struct {
	r io.Reader
}

func (f undoAGL) Read(p []byte) (int, error) {
	if len(p) == 1 {
		return 0, nil
	}
	return f.r.Read(p)
}

// NewReader returns a deterministic, cryptographically-secure source of random
// bytes seeded by `seed`.
func NewReader(seed string) (io.Reader, error) {
	sum := sha256.Sum256([]byte(seed))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}

	iv := make([]byte, block.BlockSize())
	return undoAGL{cipher.StreamReader{
		S: cipher.NewCTR(block, iv),
		R: devZero{},
	}}, nil
}
