package flip

import ()

type Secret [32]byte

func (s Secret) XOR(t Secret) Secret {
	var ret Secret
	for i, b := range s {
		ret[i] = b ^ t[i]
	}
	return ret
}
