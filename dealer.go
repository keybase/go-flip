package flip

import (
	"context"
	"math/big"
)

type GameMessageWrappedEncoded struct {
	Header UserDevice
	Body   string // base64-encoded GameMessaageBody that comes in over chat
}

type GameMessageWrapped struct {
	Header UserDevice
	Msg    GameMessage
}

type Chatter interface {
	ReadChat(context.Context) (*GameMessageWrappedEncoded, error)
	SendChat(context.Context, string) error
	ReportHook(context.Context, GameMessageWrapped)
	ResultHook(context.Context) (GameMetadata, *Result, error)
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

type GameMetadata struct {
	GameID    GameID
	Initiator UserDevice
}

type Result struct {
	P Permutation
	I []IntResult
}

func (d *Dealer) handleMessage(c context.Context, msg *GameMessageWrappedEncoded) error {
	return nil
}

func NewDealer(c Chatter) *Dealer {
	return &Dealer{chatter: c}
}

func (d *Dealer) Run(c context.Context) error {
	for {
		msg, err := d.chatter.ReadChat(c)
		if err != nil {
			return err
		}
		err = d.handleMessage(c, msg)
		if err != nil {
			return err
		}
	}
	return nil
}
