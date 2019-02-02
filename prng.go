package flip

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
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
