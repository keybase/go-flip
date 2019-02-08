package flip

import (
	"math/big"
	"time"
)

func newStart(now time.Time) Start {
	return Start{
		StartTime:            ToTime(now),
		CommitmentWindowMsec: 3 * 1000,
		RevealWindowMsec:     30 * 1000,
		SlackMsec:            1 * 1000,
	}
}

func NewStartWithBool(now time.Time) Start {
	ret := newStart(now)
	ret.Params = NewFlipParametersWithBool()
	return ret
}

func NewStartWithInt(now time.Time, mod int64) Start {
	ret := newStart(now)
	ret.Params = NewFlipParametersWithInt(mod)
	return ret
}

func NewStartWithBigInt(now time.Time, mod *big.Int) Start {
	ret := newStart(now)
	ret.Params = NewFlipParametersWithBig(mod.Bytes())
	return ret
}

func NewStartWithShuffle(now time.Time, n int64) Start {
	ret := newStart(now)
	ret.Params = NewFlipParametersWithShuffle(n)
	return ret
}
