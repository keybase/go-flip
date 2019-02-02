package flip

import (
	"math/big"
	"time"
)

type Stage int

type User string

const (
	Commit Stage = 0
	Reveal Stage = 1
)

type Input struct {
	User  User
	Data  Secret
	Stage Stage
}

type Output struct {
	Err error
	n   *big.Int
}

type userState struct {
	Commitment *Secret
	Hash       *Secret
}

type Flipper struct {
	inputCh       chan Input
	outputCh      chan Output
	modulus       *big.Int
	users         map[User](*userState)
	commitmentDur time.Duration
	revealDur     time.Duration
}

func (f *Flipper) InputChannel() <-chan Input {
	return f.inputCh
}

func (f *Flipper) OutputChannel() chan<- Output {
	return f.outputCh
}

func NewFlip(modulus *big.Int, users []User, commitmentDur time.Duration, revealDur time.Duration) *Flipper {
	d := make(map[User]*userState)
	for _, u := range users {
		d[u] = nil
	}
	return &Flipper{
		inputCh:       make(chan Input),
		outputCh:      make(chan Output),
		modulus:       modulus,
		users:         d,
		commitmentDur: commitmentDur,
		revealDur:     revealDur,
	}
}

func (f *Flipper) Run() error {
	return nil
}
