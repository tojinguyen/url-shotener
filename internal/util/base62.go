package util

import (
	"math/big"
	"strings"
)

// base62Alphabet is the ordered symbol set: digits, then upper, then lower case.
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var base62Base = big.NewInt(62)

// Base62Encode interprets b as a big-endian unsigned integer and renders it in
// base62. The result is deterministic for a given input. An all-zero or empty
// input encodes to "0".
func Base62Encode(b []byte) string {
	n := new(big.Int).SetBytes(b)
	if n.Sign() == 0 {
		return string(base62Alphabet[0])
	}

	var sb strings.Builder
	mod := new(big.Int)
	zero := big.NewInt(0)
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base62Base, mod)
		sb.WriteByte(base62Alphabet[mod.Int64()])
	}

	// DivMod yields least-significant digit first; reverse for big-endian order.
	return reverse(sb.String())
}

// reverse returns s with its bytes in reverse order. base62 output is ASCII, so
// byte-level reversal is safe.
func reverse(s string) string {
	r := []byte(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// Base62Key encodes b and returns a fixed-length key of exactly length chars.
// Shorter encodings are left-padded with the zero symbol; longer encodings are
// truncated to the leading length characters.
func Base62Key(b []byte, length int) string {
	enc := Base62Encode(b)
	if len(enc) >= length {
		return enc[:length]
	}
	return strings.Repeat(string(base62Alphabet[0]), length-len(enc)) + enc
}
