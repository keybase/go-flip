package flip

import (
	"math/big"
	"time"
)

type Stage int

const (
	Commit Stage = 0
	Reveal Stage = 1
)

type Broker struct {
	inputCh       chan Input
	outputCh      chan Output
	modulus       *big.Int
	users         map[Player](*PlayerState)
	commitmentDur time.Duration
	revealDur     time.Duration
}

type Input struct {
	Player Player
	Data   Secret
	Stage  Stage
}

type Output struct {
	Err    error
	Result *big.Int
}

func (f *Broker) InputChannel() <-chan Input {
	return f.inputCh
}

func (f *Broker) OutputChannel() chan<- Output {
	return f.outputCh
}

func NewBroker(modulus *big.Int, users []Player, commitmentDur time.Duration, revealDur time.Duration) *Broker {
	d := make(map[Player]*PlayerState)
	for _, u := range users {
		d[u] = nil
	}
	return &Broker{
		inputCh:       make(chan Input),
		outputCh:      make(chan Output),
		modulus:       modulus,
		users:         d,
		commitmentDur: commitmentDur,
		revealDur:     revealDur,
	}
}

func (f *Broker) Run() error {
	return nil
}
