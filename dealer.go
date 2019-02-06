package flip

import (
	"context"
	"encoding/base64"
	"fmt"
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

type GameStateUpdateMessage struct {
	Metatdata GameMetadata
	// only one of the three, at most, will be non-nil
	Err    error
	CC     *CommitmentComplete
	Result *Result
}

type Dealer struct {
	sync.Mutex
	dh           DealersHelper
	games        map[GameKey](chan<- *GameMessageWrapped)
	chatInputCh  chan GameMessageWrappedEncoded
	gameOutputCh chan GameStateUpdateMessage
}

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
	Shuffle []int
	Bool    *bool
	Int     *int64
	Big     *big.Int
}

type Game struct {
	id           GameID
	initiator    UserDevice
	params       Start
	key          GameKey
	msgCh        <-chan *GameMessageWrapped
	stage        Stage
	dh           DealersHelper
	players      map[UserDeviceKey]*GamePlayerState
	gameOutputCh chan GameStateUpdateMessage
	nPlayers     int
}

type GamePlayerState struct {
	ud         UserDevice
	commitment Commitment
	included   bool
	secret     *Secret
}

func (g *Game) GameMetadata() GameMetadata {
	return GameMetadata{
		Initiator: g.initiator,
		GameID:    g.id,
	}
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
		d.gameOutputCh <- GameStateUpdateMessage{
			Metatdata: game.GameMetadata(),
			Err:       err,
		}
	} else {
		d.dh.CLogf(ctx, "Game %s ended cleanly", key)
	}

	d.Lock()
	defer d.Unlock()
	close(d.games[key])
	delete(d.games, key)
}

func (g *Game) getNextTimer() <-chan time.Time {
	return g.dh.Clock().AfterTime(g.nextDeadline().Time())
}

func (g *Game) nextDeadline() Time {
	switch g.stage {
	case Stage_ROUND1:
		return g.params.CommitmentEndTime()
	case Stage_ROUND2:
		return g.params.RevealEndTime()
	default:
		return Time(0)
	}
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
	expected, err := secret.computeCommitment(g.commitmentPayload())
	if err != nil {
		return err
	}
	if ps.secret != nil {
		return DuplicateRevealError{G: g.key, U: ps.ud}
	}
	if !expected.Eq(ps.commitment) {
		return BadRevealError{G: g.key, U: ps.ud}
	}
	ps.secret = &secret
	return nil
}

func (g *Game) finishGame(ctx context.Context) error {
	var xor Secret
	for _, ps := range g.players {
		if !ps.included {
			continue
		}
		if ps.secret == nil {
			return NoRevealError{G: g.key, U: ps.ud}
		}
		xor.XOR(*ps.secret)
	}
	prng := NewPRNG(xor)
	return g.doFlip(ctx, prng)
}

func (g *Game) doFlip(ctx context.Context, prng *PRNG) error {
	params := g.params.Params
	t, err := params.T()
	if err != nil {
		return err
	}
	var res Result
	switch t {
	case FlipType_BOOL:
		tmp := prng.Bool()
		res.Bool = &tmp
	case FlipType_INT:
		tmp := prng.Int(params.Int())
		res.Int = &tmp
	case FlipType_BIG:
		var modulus big.Int
		modulus.SetBytes(params.Big())
		res.Big = prng.Big(&modulus)
	case FlipType_SHUFFLE:
		res.Shuffle = prng.Permutation(int(params.Shuffle()))
	default:
		return fmt.Errorf("unknown flip type: %s", t)
	}

	g.gameOutputCh <- GameStateUpdateMessage{
		Metatdata: g.GameMetadata(),
		Result:    &res,
	}
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
			return DuplicateRegistrationError{g.key, msg.Header}
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
				return UnregisteredUserError{G: g.key, U: u}
			}
			ps.included = true
			g.nPlayers++
		}
		g.stage = Stage_ROUND2
		g.gameOutputCh <- GameStateUpdateMessage{
			Metatdata: g.GameMetadata(),
			CC:        &cc,
		}

	case MessageType_REVEAL:
		if g.stage != Stage_ROUND2 {
			return badStage()
		}
		key := msg.Header.ToKey()
		ps := g.players[key]
		if ps == nil {
			return UnregisteredUserError{G: g.key, U: msg.Header}
		}
		if !ps.included {
			g.dh.CLogf(ctx, "Skipping unincluded sender: %s", key)
			return nil
		}
		err := g.setSecret(ctx, ps, msg.Msg.Body.Reveal())
		if err != nil {
			return err
		}
		g.nPlayers--
		if g.nPlayers == 0 {
			return g.finishGame(ctx)
		}

	default:
		return fmt.Errorf("bad message type: %d", t)
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
		id:           msg.Msg.GameID,
		initiator:    msg.Header,
		key:          key,
		params:       start,
		msgCh:        msgCh,
		stage:        Stage_ROUND1,
		dh:           d.dh,
		gameOutputCh: d.gameOutputCh,
		players:      make(map[UserDeviceKey]*GamePlayerState),
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
		return GameFinishedError{key}
	}
	game <- msg
	return nil
}

func (d *Dealer) handleMessage(c context.Context, emsg GameMessageWrappedEncoded) error {
	msg, err := emsg.Decode()
	if err != nil {
		return err
	}
	fmt.Printf("msg %+v\n", msg)
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
		gameOutputCh: make(chan GameStateUpdateMessage, 5),
	}
}

func (d *Dealer) UpdateCh() <-chan GameStateUpdateMessage {
	return d.gameOutputCh
}

func (d *Dealer) MessageCh() chan<- GameMessageWrappedEncoded {
	return d.chatInputCh
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
		}
	}
	return nil
}
