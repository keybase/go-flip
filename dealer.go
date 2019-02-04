package flip

import (
	"context"
	"encoding/base64"
	clockwork "github.com/keybase/clockwork"
	"github.com/keybase/go-codec/codec"
	"math/big"
	"strings"
	"sync"
	"time"
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
	CLogf(ctx context.Context, fmt string, args ...interface{})
	Clock() clockwork.Clock
}

type Dealer struct {
	sync.Mutex
	chatter Chatter
	games   map[GameKey](chan<- *GameMessageWrapped)
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

func (g GameMessageWrapped) GameMetadata() GameMetadata {
	return GameMetadata{GameID: g.Msg.GameID, Initiator: g.Header}
}

func (g GameMetadata) ToKey() GameKey {
	return GameKey(strings.Join([]string{g.Initiator.User.String(), g.Initiator.Device.String(), g.GameID.String()}, ","))
}

type GameKey string

type Result struct {
	P Permutation
	I []IntResult
}

type Game struct {
	params  Start
	key     GameKey
	msgCh   <-chan *GameMessageWrapped
	stage   Stage
	chatter Chatter
}

func codecHandle() *codec.MsgpackHandle {
	var mh codec.MsgpackHandle
	mh.WriteExt = true
	return &mh
}

func msgpackDecode(dst interface{}, src []byte) error {
	h := codecHandle()
	return codec.NewDecoderBytes(src, h).Decode(dst)
}

func (e *GameMessageWrappedEncoded) Decode() (*GameMessageWrapped, error) {
	raw, err := base64.StdEncoding.DecodeString(e.Body)
	if err != nil {
		return nil, err
	}
	var msg GameMessage
	err = msgpackDecode(&msg, raw)
	if err != nil {
		return nil, err
	}
	ret := GameMessageWrapped{Header: e.Header, Msg: msg}
	return &ret, nil
}

func (d *Dealer) run(ctx context.Context, game *Game) {
	doneCh := make(chan error)
	key := game.key
	go func() {
		doneCh <- game.run(ctx)
	}()
	err := <-doneCh
	if err != nil {
		d.chatter.CLogf(ctx, "Error running game %s: %s", key, err.Error())
	} else {
		d.chatter.CLogf(ctx, "Game %s ended cleanly", key)
	}
	d.Lock()
	defer d.Unlock()
	close(d.games[key])
	delete(d.games, key)
}

func (g *Game) getNextTimer() <-chan time.Time {
	return g.chatter.Clock().AfterTime(g.nextDeadline())
}

func (g *Game) nextDeadline() time.Time {
	return time.Time{}
}

func (g *Game) handleMessage(ctx context.Context, msg *GameMessageWrapped) error {
	s, err := msg.Msg.Body.S()
	if err != nil {
		return err
	}
	switch s {
	case Stage_START:
		return BadMessageForStageError{MessageStage: s, GameStage: g.stage}
	case Stage_REGISTER:
	}
	return nil
}

func (g *Game) run(ctx context.Context) error {
	for {
		timer := g.getNextTimer()
		select {
		case <-timer:
			return TimeoutError{Key: g.key, Stage: g.stage}
		case msg := <-g.msgCh:
			err := g.handleMessage(ctx, msg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Dealer) handleMessageStart(ctx context.Context, msg *GameMessageWrapped, start Start) error {
	d.Lock()
	defer d.Unlock()
	key := msg.GameMetadata().ToKey()
	if d.games[key] != nil {
		return GameAlreadyStartedError(key)
	}
	msgCh := make(chan *GameMessageWrapped)
	game := &Game{
		key:     key,
		params:  start,
		msgCh:   msgCh,
		stage:   Stage_START,
		chatter: d.chatter,
	}
	d.games[key] = msgCh
	go d.run(ctx, game)
	return nil
}

func (d *Dealer) handleMessageOthers(c context.Context, msg *GameMessageWrapped) error {
	d.Lock()
	defer d.Unlock()
	key := msg.GameMetadata().ToKey()
	game := d.games[key]
	if game == nil {
		return GameFinishedError(key)
	}
	game <- msg
	return nil
}

func (d *Dealer) handleMessage(c context.Context, emsg *GameMessageWrappedEncoded) error {
	msg, err := emsg.Decode()
	if err != nil {
		return err
	}
	s, err := msg.Msg.Body.S()
	if err != nil {
		return err
	}
	switch s {
	case Stage_START:
		return d.handleMessageStart(c, msg, msg.Msg.Body.Start())
	default:
		return d.handleMessageOthers(c, msg)
	}
	return nil
}

func NewDealer(c Chatter) *Dealer {
	return &Dealer{
		chatter: c,
		games:   make(map[GameKey](chan<- *GameMessageWrapped)),
	}
}

func (d *Dealer) Run(ctx context.Context) error {
	for {
		msg, err := d.chatter.ReadChat(ctx)
		if err != nil {
			return err
		}
		err = d.handleMessage(ctx, msg)
		if err != nil {
			d.chatter.CLogf(ctx, "Error reading message: %s", err.Error())
			return err
		}
	}
	return nil
}
