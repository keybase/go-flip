package flip

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
)

type PRNG struct {
	key Secret
	buf []byte
	i   uint64
}

func NewPRNG(s Secret) *PRNG {
	return &PRNG{
		key: s,
		i:   uint64(1),
	}
}

func uint64ToSlice(i uint64) []byte {
	var ret [8]byte
	binary.BigEndian.PutUint64(ret[:], i)
	return ret[:]
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func (p *PRNG) read(ret []byte) int {
	n := min(len(p.buf), len(ret))
	copy(ret[0:n], p.buf[0:n])
	p.buf = p.buf[n:]
	return n
}

func (p *PRNG) replenish() {
	if len(p.buf) == 0 {
		p.buf = hmac.New(sha256.New, p.key[:]).Sum(uint64ToSlice(p.i))
		p.i++
	}
}

func (p *PRNG) Read(out []byte) int {
	var nRead int
	for nRead < len(out) {
		p.replenish()
		tmp := p.read(out[nRead:])
		nRead += tmp
	}
	return nRead
}

func (p *PRNG) NextModN(n *big.Int) *big.Int {
	bits := n.BitLen()

	// Compute the number of bytes it takes to get that many bits.
	// but rounding up.
	bytes := bits / 8
	if bits%8 != 0 {
		bytes++
	}

	buf := make([]byte, bytes)
	for {
		p.Read(buf)
		x := big.NewInt(0)
		x.SetBytes(buf)
		x.Rsh(x, uint(bytes*8-bits))
		if x.Cmp(n) < 0 {
			return x
		}
	}
	return nil
}
