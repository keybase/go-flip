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
	Sender UserDevice
	Body   string // base64-encoded GameMessaageBody that comes in over chat
}

type GameMessageWrapped struct {
	Sender UserDevice
	Msg    GameMessageV1
}

type DealersHelper interface {
	CLogf(ctx context.Context, fmt string, args ...interface{})
	Clock() clockwork.Clock
	ServerTime(context.Context) (time.Time, error)
	ReadHistory(ctx context.Context, since time.Time) ([]GameMessageWrappedEncoded, error)
	Me() UserDevice
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
	dh            DealersHelper
	games         map[GameKey](chan<- *GameMessageWrapped)
	chatInputCh   chan GameMessageWrappedEncoded
	gameOutputCh  chan GameStateUpdateMessage
	previousGames map[GameIDKey]bool
}

func (g GameMessageWrapped) GameMetadata() GameMetadata {
	return g.Msg.Md
}

func (g GameMetadata) ToKey() GameKey {
	return GameKey(strings.Join([]string{g.Initiator.U.String(), g.Initiator.D.String(), g.ChannelID.String(), g.GameID.String()}, ","))
}

func (g GameMetadata) String() string {
	return string(g.ToKey())
}

type GameKey string
type GameIDKey string
type UserDeviceKey string

func (u UserDevice) ToKey() UserDeviceKey {
	return UserDeviceKey(strings.Join([]string{u.U.String(), u.D.String()}, ","))
}

func (g GameID) ToKey() GameIDKey {
	return GameIDKey(g.String())
}

type Result struct {
	Shuffle []int
	Bool    *bool
	Int     *int64
	Big     *big.Int
}

type Game struct {
	md           GameMetadata
	clockSkew    time.Duration
	start        time.Time
	isLeader     bool
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
	ud             UserDevice
	commitment     Commitment
	commitmentTime time.Time
	included       bool
	secret         *Secret
}

func (g *Game) GameMetadata() GameMetadata {
	return g.md
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
	ret := GameMessageWrapped{Sender: e.Sender, Msg: msg.V1()}
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
	return g.dh.Clock().AfterTime(g.nextDeadline())
}

func (g *Game) CommitmentEndTime() time.Time {
	// If we're the leader, then let's cut off when we say we're going to cut off
	// If we're not, then let's give extra time (a multiple of 2) to the leader.
	mul := time.Duration(1)
	if !g.isLeader {
		mul = time.Duration(2)
	}
	return g.start.Add(mul * g.params.CommitmentWindowWithSlack())
}

func (g *Game) RevealEndTime() time.Time {
	return g.start.Add(g.params.RevealWindowWithSlack())
}

func (g *Game) nextDeadline() time.Time {
	switch g.stage {
	case Stage_ROUND1:
		return g.CommitmentEndTime()
	case Stage_ROUND2:
		return g.RevealEndTime()
	default:
		return time.Time{}
	}
}

func (g Game) commitmentPayload() CommitmentPayload {
	return CommitmentPayload{
		V: Version_V1,
		U: g.md.Initiator.U,
		D: g.md.Initiator.D,
		C: g.md.ChannelID,
		G: g.md.GameID,
		S: g.params.StartTime,
	}
}

func (g *Game) setSecret(ctx context.Context, ps *GamePlayerState, secret Secret) error {
	expected, err := secret.computeCommitment(g.commitmentPayload())
	if err != nil {
		return err
	}
	if ps.secret != nil {
		return DuplicateRevealError{G: g.md, U: ps.ud}
	}
	if !expected.Eq(ps.commitment) {
		return BadRevealError{G: g.md, U: ps.ud}
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
			return NoRevealError{G: g.md, U: ps.ud}
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

func (g *Game) playerCommitedInTime(ps *GamePlayerState, now time.Time) bool {
	diff := ps.commitmentTime.Sub(g.start)
	return diff < g.params.CommitmentWindowWithSlack()
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
		key := msg.Sender.ToKey()
		if g.players[key] != nil {
			return DuplicateRegistrationError{g.md, msg.Sender}
		}
		g.players[key] = &GamePlayerState{
			ud:             msg.Sender,
			commitment:     msg.Msg.Body.Commitment(),
			commitmentTime: g.dh.Clock().Now(),
		}

	case MessageType_COMMITMENT_COMPLETE:
		if g.stage != Stage_ROUND1 {
			return badStage()
		}
		if !msg.Sender.Eq(g.md.Initiator) {
			return WrongSenderError{G: g.md, Expected: g.md.Initiator, Actual: msg.Sender}
		}
		now := g.dh.Clock().Now()
		cc := msg.Msg.Body.CommitmentComplete()
		for _, u := range cc.Players {
			key := u.ToKey()
			ps := g.players[key]
			if ps == nil {
				return UnregisteredUserError{G: g.md, U: u}
			}
			ps.included = true
			g.nPlayers++
		}

		// for now, just warn if users who made it in on time weren't included.
		for _, ps := range g.players {
			if !ps.included && g.playerCommitedInTime(ps, now) {
				g.dh.CLogf(ctx, "User %s wasn't included, but they should have been", ps.ud)
			}
		}

		g.commitRound2(&cc)

	case MessageType_REVEAL:
		if g.stage != Stage_ROUND2 {
			return badStage()
		}
		key := msg.Sender.ToKey()
		ps := g.players[key]
		if ps == nil {
			return UnregisteredUserError{G: g.md, U: msg.Sender}
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

func (g *Game) commitRound2(cc *CommitmentComplete) {
	g.stage = Stage_ROUND2
	g.gameOutputCh <- GameStateUpdateMessage{
		Metatdata: g.GameMetadata(),
		CC:        cc,
	}
}

func (g *Game) completeCommitments(ctx context.Context) error {
	var cc CommitmentComplete
	for _, p := range g.players {
		if p.secret == nil {
			continue
		}
		cc.Players = append(cc.Players, p.ud)
		p.included = true
		g.nPlayers++
	}
	g.commitRound2(&cc)
	return nil
}

func (g *Game) handleTimerEvent(ctx context.Context) error {
	if g.isLeader && g.stage == Stage_ROUND1 {
		return g.completeCommitments(ctx)
	}
	return TimeoutError{G: g.md, Stage: g.stage}
}

func (g *Game) run(ctx context.Context) error {
	for {
		timer := g.getNextTimer()
		var err error
		select {
		case <-timer:
			err = g.handleTimerEvent(ctx)
		case msg := <-g.msgCh:
			err = g.handleMessage(ctx, msg)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func absDuration(d time.Duration) time.Duration {
	if d < time.Duration(0) {
		return time.Duration(-1) * d
	}
	return d
}

func (d *Dealer) computeClockSkew(ctx context.Context, md GameMetadata, leaderTime time.Time) (skew time.Duration, err error) {
	serverTime, err := d.dh.ServerTime(ctx)
	localTime := d.dh.Clock().Now()

	leaderSkew := leaderTime.Sub(serverTime)
	localSkew := localTime.Sub(serverTime)

	if absDuration(localSkew) > MaxClockSkew {
		return time.Duration(0), BadLocalClockError{G: md}
	}
	if absDuration(leaderSkew) > MaxClockSkew {
		return time.Duration(0), BadLeaderClockError{G: md}
	}
	totalSkew := localTime.Sub(leaderTime)

	return totalSkew, nil
}

func (d *Dealer) primeHistory(ctx context.Context) (err error) {
	if d.previousGames != nil {
		return nil
	}

	tmp := make(map[GameIDKey]bool)
	prevs, err := d.dh.ReadHistory(ctx, d.dh.Clock().Now().Add(time.Duration(-2)*MaxClockSkew))
	if err != nil {
		return err
	}
	for _, m := range prevs {
		gmw, err := m.Decode()
		if err != nil {
			return err
		}
		tmp[gmw.Msg.Md.GameID.ToKey()] = true
	}
	d.previousGames = tmp
	return nil
}

func (d *Dealer) handleMessageStart(ctx context.Context, msg *GameMessageWrapped, start Start) error {
	d.Lock()
	defer d.Unlock()
	md := msg.GameMetadata()
	key := md.ToKey()
	if d.games[key] != nil {
		return GameAlreadyStartedError{G: md}
	}
	if !msg.Sender.Eq(md.Initiator) {
		return WrongSenderError{G: md, Expected: msg.Sender, Actual: md.Initiator}
	}
	cs, err := d.computeClockSkew(ctx, md, start.StartTime.Time())
	if err != nil {
		return err
	}

	err = d.primeHistory(ctx)
	if err != nil {
		return err
	}

	if d.previousGames[md.GameID.ToKey()] {
		return GameReplayError{md.GameID}
	}

	msgCh := make(chan *GameMessageWrapped)
	game := &Game{
		md:           msg.GameMetadata(),
		isLeader:     d.dh.Me().Eq(msg.Sender),
		clockSkew:    cs,
		start:        d.dh.Clock().Now(),
		key:          key,
		params:       start,
		msgCh:        msgCh,
		stage:        Stage_ROUND1,
		dh:           d.dh,
		gameOutputCh: d.gameOutputCh,
		players:      make(map[UserDeviceKey]*GamePlayerState),
	}
	d.games[key] = msgCh
	d.previousGames[md.GameID.ToKey()] = true
	go d.run(ctx, game)
	return nil
}

func (d *Dealer) handleMessageOthers(c context.Context, msg *GameMessageWrapped) error {
	d.Lock()
	defer d.Unlock()
	md := msg.GameMetadata()
	key := md.ToKey()
	game := d.games[key]
	if game == nil {
		return GameFinishedError{G: md}
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

func (d *Dealer) Stop() {
	close(d.chatInputCh)
}
