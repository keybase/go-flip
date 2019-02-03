package flip

import (
	"math/big"
	"time"
)

type Broker struct {
	inputCh       chan Input
	outputCh      chan Output
	modulus       *big.Int
	players       map[Player](*PlayerState)
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

func NewBroker(modulus *big.Int, players []Player, commitmentDur time.Duration, revealDur time.Duration) *Broker {
	d := make(map[Player]*PlayerState)
	for _, p := range players {
		d[p] = nil
	}
	return &Broker{
		inputCh:       make(chan Input),
		outputCh:      make(chan Output),
		modulus:       modulus,
		players:       d,
		commitmentDur: commitmentDur,
		revealDur:     revealDur,
	}
}

func (f *Broker) Run() error {
	return nil
}
