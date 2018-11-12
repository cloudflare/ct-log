// Package frontier implements tools for protecting the right-most edge of a CT
// log's Merkle tree.
package frontier

import (
	"crypto/sha256"
)

// Frontier stores the roots of the right-most perfect Merkle trees (that is,
// Merkle trees with 2^n leaves) of a CT log. It is compact, and used for
// preventing storage bugs from damaging the log.
type Frontier struct {
	Roots [][]byte
}

// Append adds the merkle hash `h` as the right-most node of the tree.
func (f *Frontier) Append(h []byte) {
	carry := h
	for i, _ := range f.Roots {
		if f.Roots[i] == nil {
			f.Roots[i] = carry
			carry = nil
			break
		}

		carry = hashDomain(0x01, f.Roots[i], carry)
		f.Roots[i] = nil
	}
	if carry != nil {
		f.Roots = append(f.Roots, carry)
	}
}

func (f *Frontier) Head() []byte {
	if len(f.Roots) == 0 {
		empty := sha256.Sum256(nil)
		return empty[:]
	}

	var acc []byte
	for i := 0; i < len(f.Roots); i++ {
		if f.Roots[i] == nil {
			continue
		} else if acc == nil {
			acc = f.Roots[i]
			continue
		}
		acc = hashDomain(0x01, f.Roots[i], acc)
	}

	return acc
}

func hashDomain(domain byte, slices ...[]byte) []byte {
	in := make([]byte, 1)
	in[0] = domain

	for _, slice := range slices {
		in = append(in, slice...)
	}

	out := sha256.Sum256(in)
	return out[:]
}
