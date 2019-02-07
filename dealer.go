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

type GameMessageEncoded string

type GameMessageWrappedEncoded struct {
	Sender UserDevice
	Body   GameMessageEncoded // base64-encoded GameMessaageBody that comes in over chat
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
	SendChat(ctx context.Context, ch ChannelID, msg GameMessageEncoded) error
	Me() UserDevice
}

type GameStateUpdateMessage struct {
	Metadata GameMetadata
	// only one of the following will be non-nil
	Err                error
	Commitment         *UserDevice
	CommitmentComplete *CommitmentComplete
	Result             *Result
}

type Dealer struct {
	sync.Mutex
	dh            DealersHelper
	games         map[GameKey](chan<- *GameMessageWrapped)
	shutdownCh    chan struct{}
	chatInputCh   chan *GameMessageWrapped
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
	md              GameMetadata
	clockSkew       time.Duration
	start           time.Time
	isLeader        bool
	params          Start
	key             GameKey
	msgCh           <-chan *GameMessageWrapped
	stage           Stage
	stageForTimeout Stage
	dh              DealersHelper
	players         map[UserDeviceKey]*GamePlayerState
	gameOutputCh    chan GameStateUpdateMessage
	nPlayers        int
	dealer          *Dealer
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

func (e GameMessageEncoded) Decode() (*GameMessageV1, error) {
	raw, err := base64.StdEncoding.DecodeString(string(e))
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
	tmp := msg.V1()
	return &tmp, nil
}

func (e *GameMessageWrappedEncoded) Decode() (*GameMessageWrapped, error) {
	v1, err := e.Body.Decode()
	if err != nil {
		return nil, err
	}
	ret := GameMessageWrapped{Sender: e.Sender, Msg: *v1}
	return &ret, nil
}

func (w GameMessageWrapped) Encode() (GameMessageEncoded, error) {
	return w.Msg.Encode()
}

func (b GameMessageBody) Encode(md GameMetadata) (GameMessageEncoded, error) {
	v1 := GameMessageV1{Md: md, Body: b}
	return v1.Encode()
}

func (v GameMessageV1) Encode() (GameMessageEncoded, error) {
	msg := NewGameMessageWithV1(v)
	raw, err := msgpackEncode(msg)
	if err != nil {
		return GameMessageEncoded(""), err
	}
	return GameMessageEncoded(base64.StdEncoding.EncodeToString(raw)), nil
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
			Metadata: game.GameMetadata(),
			Err:      err,
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
	switch g.stageForTimeout {
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
		Metadata: g.GameMetadata(),
		Result:   &res,
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
		return BadMessageForStageError{G: g.GameMetadata(), MessageType: t, Stage: g.stage}
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
		g.gameOutputCh <- GameStateUpdateMessage{
			Metadata:   g.GameMetadata(),
			Commitment: &msg.Sender,
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
		g.stage = Stage_ROUND2
		g.stageForTimeout = Stage_ROUND2
		g.gameOutputCh <- GameStateUpdateMessage{
			Metadata:           g.GameMetadata(),
			CommitmentComplete: &cc,
		}

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

func (g *Game) completeCommitments(ctx context.Context) error {
	var cc CommitmentComplete
	for _, p := range g.players {
		cc.Players = append(cc.Players, p.ud)
	}
	body := NewGameMessageBodyWithCommitmentComplete(cc)
	g.stageForTimeout = Stage_ROUND2
	g.sendOutgoingChat(ctx, body)
	return nil
}

func (g *Game) sendOutgoingChat(ctx context.Context, body GameMessageBody) {
	// Call back into the dealer, to reroute a message back into our
	// game, but do so in a Go routine so we don't deadlock.
	go g.dealer.sendOutgoingChat(ctx, g.GameMetadata(), body)
}

func (g *Game) handleTimerEvent(ctx context.Context) error {
	if g.isLeader && g.stageForTimeout == Stage_ROUND1 {
		return g.completeCommitments(ctx)
	}
	return TimeoutError{G: g.md, Stage: g.stageForTimeout}
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
		md:              msg.GameMetadata(),
		isLeader:        d.dh.Me().Eq(msg.Sender),
		clockSkew:       cs,
		start:           d.dh.Clock().Now(),
		key:             key,
		params:          start,
		msgCh:           msgCh,
		stage:           Stage_ROUND1,
		stageForTimeout: Stage_ROUND1,
		dh:              d.dh,
		gameOutputCh:    d.gameOutputCh,
		players:         make(map[UserDeviceKey]*GamePlayerState),
		dealer:          d,
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

func (d *Dealer) handleMessage(c context.Context, msg *GameMessageWrapped) error {
	fmt.Printf("msg %+v\n", *msg)
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
		shutdownCh:   make(chan struct{}),
		chatInputCh:  make(chan *GameMessageWrapped),
		gameOutputCh: make(chan GameStateUpdateMessage, 500),
	}
}

func (d *Dealer) UpdateCh() <-chan GameStateUpdateMessage {
	return d.gameOutputCh
}

func (d *Dealer) MessageCh() chan<- *GameMessageWrapped {
	return d.chatInputCh
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
}

type PlayerControl struct {
	me         UserDevice
	md         GameMetadata
	secret     Secret
	commitment Commitment
	start      Start
	dealer     *Dealer
}

func (d *Dealer) newPlayerControl(me UserDevice, md GameMetadata, start Start) (*PlayerControl, error) {
	secret := GenerateSecret()
	cp := CommitmentPayload{
		V: Version_V1,
		U: md.Initiator.U,
		D: md.Initiator.D,
		C: md.ChannelID,
		G: md.GameID,
		S: start.StartTime,
	}
	commitment, err := secret.computeCommitment(cp)
	if err != nil {
		return nil, err
	}
	return &PlayerControl{
		me:         me,
		md:         md,
		secret:     secret,
		commitment: commitment,
		start:      start,
		dealer:     d,
	}, nil
}

func (p *PlayerControl) GameMetadata() GameMetadata {
	return p.md
}

func (d *Dealer) StartFlip(ctx context.Context, start Start, chid ChannelID) (pc *PlayerControl, err error) {
	md := GameMetadata{
		Initiator: d.dh.Me(),
		ChannelID: chid,
		GameID:    GenerateGameID(),
	}
	pc, err = d.newPlayerControl(d.dh.Me(), md, start)
	if err != nil {
		return nil, err
	}
	err = d.sendOutgoingChat(ctx, md, NewGameMessageBodyWithStart(start))
	if err != nil {
		return nil, err
	}
	err = d.sendOutgoingChat(ctx, md, NewGameMessageBodyWithCommitment(pc.commitment))
	if err != nil {
		return nil, err
	}
	return pc, nil
}

func (d *Dealer) sendOutgoingChat(ctx context.Context, md GameMetadata, body GameMessageBody) error {

	gmw := GameMessageWrapped{
		Sender: d.dh.Me(),
		Msg: GameMessageV1{
			Md:   md,
			Body: body,
		},
	}

	// Reinject the message into the state machine.
	d.chatInputCh <- &gmw

	// Encode and send the message through the external server-routed chat channel
	emsg, err := gmw.Encode()
	if err != nil {
		return err
	}
	return d.dh.SendChat(ctx, md.ChannelID, emsg)
}

func (d *Dealer) InjectIncomingChat(ctx context.Context, sender UserDevice, body GameMessageEncoded) error {
	gmwe := GameMessageWrappedEncoded{
		Sender: sender,
		Body:   body,
	}
	msg, err := gmwe.Decode()
	if err != nil {
		return err
	}
	d.chatInputCh <- msg
	return nil
}
