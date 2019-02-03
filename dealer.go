package flip

import (
	"context"
	"math/big"
)

type GameMessageHeader struct {
	User   UID
	Device DeviceID
}

type GameMessageEncoded struct {
	Header GameMessageHeader
	Body   string // base64-encoded GameMessaageBody that comes in over chat
}

type GameMessage struct {
	Header GameMessageHeader
	Body   GameMessageBody
}

type Chatter interface {
	ReadNextMessage(c context.Context) (*GameMessageEncoded, error)
	SendMessage(c context.Context, gm GameMessageEncoded) error
	RepoortHook(c context.Context, gm GameMessage) error
}

type Dealer struct {
	chatter Chatter
}

type IntResult struct {
	b   *bool
	i   *int64
	big *big.Int
}

type Permutation []int

type Result struct {
	P Permutation
	I []IntResult
}

type res struct {
	p *FlipParameters
	r *Result
	e error
}

func (d *Dealer) runLoop(c context.Context, doneCh chan<- res) {
}

func (d *Dealer) Run(c context.Context) (*FlipParameters, *Result, error) {
	doneCh := make(chan res)
	go d.runLoop(c, doneCh)
	res := <-doneCh
	return res.p, res.r, res.e
}
