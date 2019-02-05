package flip

import (
	"context"
	"encoding/base64"
	clockwork "github.com/keybase/clockwork"
	"io"
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
	Msg    GameMessageV1
}

type DealersHelper interface {
	CLogf(ctx context.Context, fmt string, args ...interface{})
	Clock() clockwork.Clock
}

type GameStateUpdateMesasge struct {
	Metatdata GameMetadata
	Err       error
}

type Dealer struct {
	sync.Mutex
	dh           DealersHelper
	games        map[GameKey](chan<- *GameMessageWrapped)
	chatInputCh  chan GameMessageWrappedEncoded
	gameOutputCh chan GameStateUpdateMesasge
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
	return GameKey(strings.Join([]string{g.Initiator.U.String(), g.Initiator.D.String(), g.GameID.String()}, ","))
}

type GameKey string
type UserDeviceKey string

func (u UserDevice) ToKey() UserDeviceKey {
	return UserDeviceKey(strings.Join([]string{u.U.String(), u.D.String()}, ","))
}

type Result struct {
	P Permutation
	I []IntResult
}

type Game struct {
	id        GameID
	initiator UserDevice
	params    Start
	key       GameKey
	msgCh     <-chan *GameMessageWrapped
	stage     Stage
	dh        DealersHelper
	players   map[UserDeviceKey]*GamePlayerState
}

type GamePlayerState struct {
	ud         UserDevice
	commitment Commitment
	included   bool
	secret     *Secret
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
	v, err := msg.V()
	if err != nil {
		return nil, err
	}
	if v != Version_V1 {
		return nil, BadVersionError(v)
	}
	ret := GameMessageWrapped{Header: e.Header, Msg: msg.V1()}
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
		d.dh.CLogf(ctx, "Error running game %s: %s", key, err.Error())
	} else {
		d.dh.CLogf(ctx, "Game %s ended cleanly", key)
	}
	d.Lock()
	defer d.Unlock()
	close(d.games[key])
	delete(d.games, key)
}

func (g *Game) getNextTimer() <-chan time.Time {
	return g.dh.Clock().AfterTime(g.nextDeadline())
}

func (g *Game) nextDeadline() time.Time {
	return time.Time{}
}

func (g Game) commitmentPayload() CommitmentPayload {
	return CommitmentPayload{
		V: Version_V1,
		U: g.initiator,
		I: g.id,
		S: g.params.StartTime,
	}
}

func (g *Game) setSecret(ctx context.Context, ps *GamePlayerState, secret Secret) error {
	key := ps.ud.ToKey()
	expected, err := secret.computeCommitment(g.commitmentPayload())
	if err != nil {
		return err
	}
	if ps.secret != nil {
		return DuplicateRevealError{G: g.key, U: key}
	}
	if !expected.Eq(ps.commitment) {
		return BadRevealError{G: g.key, U: key}
	}
	ps.secret = &secret
	return nil
}

func (g *Game) handleMessage(ctx context.Context, msg *GameMessageWrapped) error {
	t, err := msg.Msg.Body.T()
	if err != nil {
		return err
	}
	badStage := func() error {
		return BadMessageForStageError{MessageType: t, Stage: g.stage}
	}
	switch t {
	case MessageType_START:
		return badStage()
	case MessageType_COMMITMENT:
		if g.stage != Stage_ROUND1 {
			return badStage()
		}
		key := msg.Header.ToKey()
		if g.players[key] != nil {
			return DuplicateRegistrationError{g.key, key}
		}
		g.players[key] = &GamePlayerState{
			ud:         msg.Header,
			commitment: msg.Msg.Body.Commitment(),
		}
	case MessageType_COMMITMENT_COMPLETE:
		if g.stage != Stage_ROUND1 {
			return badStage()
		}
		if !msg.Header.Eq(g.initiator) {
			return WrongSenderError{G: g.key, Expected: g.initiator, Actual: msg.Header}
		}
		cc := msg.Msg.Body.CommitmentComplete()
		for _, u := range cc.Players {
			key := u.ToKey()
			ps := g.players[key]
			if ps == nil {
				return UnregisteredUserError{G: g.key, U: key}
			}
			ps.included = true
		}
		g.stage = Stage_ROUND2
	case MessageType_REVEAL:
		if g.stage != Stage_ROUND2 {
			return badStage()
		}
		key := msg.Header.ToKey()
		ps := g.players[key]
		if ps == nil {
			return UnregisteredUserError{G: g.key, U: key}
		}
		if !ps.included {
			g.dh.CLogf(ctx, "Skipping unincluded sender: %s", key)
			return nil
		}
		err := g.setSecret(ctx, ps, msg.Msg.Body.Reveal())
		if err != nil {
			return err
		}
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
		id:        msg.Msg.GameID,
		initiator: msg.Header,
		key:       key,
		params:    start,
		msgCh:     msgCh,
		stage:     Stage_ROUND1,
		dh:        d.dh,
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

func (d *Dealer) handleMessage(c context.Context, emsg GameMessageWrappedEncoded) error {
	msg, err := emsg.Decode()
	if err != nil {
		return err
	}
	t, err := msg.Msg.Body.T()
	if err != nil {
		return err
	}
	switch t {
	case MessageType_START:
		return d.handleMessageStart(c, msg, msg.Msg.Body.Start())
	default:
		return d.handleMessageOthers(c, msg)
	}
	return nil
}

func NewDealer(dh DealersHelper) *Dealer {
	return &Dealer{
		dh:           dh,
		games:        make(map[GameKey](chan<- *GameMessageWrapped)),
		chatInputCh:  make(chan GameMessageWrappedEncoded),
		gameOutputCh: make(chan GameStateUpdateMesasge),
	}
}

func (d *Dealer) UpdateCh() <-chan GameStateUpdateMesasge {
	return d.gameOutputCh
}

func (d *Dealer) Run(ctx context.Context) error {
	for {
		var msg GameMessageWrappedEncoded
		var ok bool
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok = <-d.chatInputCh:
			if !ok {
				return io.EOF
			}
		}
		err := d.handleMessage(ctx, msg)
		if err != nil {
			d.dh.CLogf(ctx, "Error reading message: %s", err.Error())
			return err
		}
	}
	return nil
}
