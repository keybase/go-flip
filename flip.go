
package flip

import (
	"context"
	clockwork "github.com/keybase/clockwork"
	"io"
	"math/big"
	"sync"
	"time"
)

type GameMessageEncoded string

type GameMessageWrappedEncoded struct {
	Sender UserDevice
	Body   GameMessageEncoded // base64-encoded GameMessaageBody that comes in over chat
}

type GameMessageWrapped struct {
	Sender  UserDevice
	Msg     GameMessageV1
	Me      *playerControl
	Forward bool
}


type GameStateUpdateMessage struct {
	Metadata GameMetadata
	// only one of the following will be non-nil
	Err                error
	Commitment         *UserDevice
	Reveal             *UserDevice
	CommitmentComplete *CommitmentComplete
	Result             *Result
}

type Dealer struct {
	sync.Mutex
	dh            DealersHelper
	games         map[GameKey](chan<- *GameMessageWrapped)
	shutdownCh    chan struct{}
	chatInputCh   chan *GameMessageWrapped
	gameUpdateCh  chan GameStateUpdateMessage
	previousGames map[GameIDKey]bool
}


type DealersHelper interface {
	CLogf(ctx context.Context, fmt string, args ...interface{})
	Clock() clockwork.Clock
	ServerTime(context.Context) (time.Time, error)
	ReadHistory(ctx context.Context, since time.Time) ([]GameMessageWrappedEncoded, error)
	SendChat(ctx context.Context, ch ConversationID, msg GameMessageEncoded) error
	Me() UserDevice
}

func NewDealer(dh DealersHelper) *Dealer {
	return &Dealer{
		dh:           dh,
		games:        make(map[GameKey](chan<- *GameMessageWrapped)),
		shutdownCh:   make(chan struct{}),
		chatInputCh:  make(chan *GameMessageWrapped),
		gameUpdateCh: make(chan GameStateUpdateMessage, 500),
	}
}

func (d *Dealer) UpdateCh() <-chan GameStateUpdateMessage {
	return d.gameUpdateCh
}

func (d *Dealer) Run(ctx context.Context) error {
	for {
		select {

		case <-ctx.Done():
			return ctx.Err()

			// This channel never closes
		case msg := <-d.chatInputCh:
			err := d.handleMessage(ctx, msg)
			if err != nil {
				d.dh.CLogf(ctx, "Error reading message: %s", err.Error())
			}

			// exit the loop if we've shutdown
		case <-d.shutdownCh:
			return io.EOF

		}
	}
	return nil
}

func (d *Dealer) Stop() {
	close(d.shutdownCh)
	d.stopGames()
}

func (d *Dealer) StartFlip(ctx context.Context, start Start, conversationID ConversationID) (err error) {
	_, err = d.startFlip(ctx, start, conversationID)
	return err
}


func (d *Dealer) InjectIncomingChat(ctx context.Context, sender UserDevice, conversationID ConversationID, body GameMessageEncoded) error {
	gmwe := GameMessageWrappedEncoded{
		Sender: sender,
		Body:   body,
	}
	msg, err := gmwe.Decode()
	if err != nil {
		return err
	}
	if !msg.Msg.Md.ConversationID.Eq(conversationID) {
		return BadChannelError{G: msg.Msg.Md, C: conversationID}
	}
	if !msg.isForwardable() {
		return UnforwardableMessageError{G: msg.Msg.Md}
	}
	d.chatInputCh <- msg
	return nil
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
