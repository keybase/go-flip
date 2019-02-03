package flip

import (
	"crypto/hmac"
	"crypto/sha256"
)

func (s *Secret) XOR(t Secret) *Secret {
	for i, b := range t {
		s[i] = b ^ s[i]
	}
	return s
}

func (s Secret) IsNil() bool {
	for _, b := range s[:] {
		if b != byte(0) {
			return false
		}
	}
	return true
}

func (s Secret) Hash() Secret {
	h := sha256.New()
	h.Write(s[:])
	tmp := h.Sum(nil)
	var ret Secret
	copy(ret[:], tmp[:])
	return ret
}

func (s Secret) Eq(t Secret) bool {
	return hmac.Equal(s[:], t[:])
}
