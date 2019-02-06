package flip

import (
	"context"
	"crypto/rand"
	"fmt"
	clockwork "github.com/keybase/clockwork"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testDealersHelper struct {
	clock clockwork.FakeClock
	me    UserDevice
}

func newTestDealersHelper() *testDealersHelper {
	return &testDealersHelper{clock: clockwork.NewFakeClock()}
}

func (t *testDealersHelper) Clock() clockwork.Clock {
	return t.clock
}

func (t *testDealersHelper) ServerTime(context.Context) (time.Time, error) {
	return t.clock.Now(), nil
}

func (t *testDealersHelper) CLogf(ctx context.Context, fmtString string, args ...interface{}) {
	fmt.Printf(fmtString+"\n", args...)
}

func (t *testDealersHelper) ReadHistory(ctx context.Context, since time.Time) ([]GameMessageWrappedEncoded, error) {
	return nil, nil
}

func (t *testDealersHelper) Me() UserDevice {
	return t.me
}

func randBytes(i int) []byte {
	ret := make([]byte, i)
	rand.Read(ret[:])
	return ret
}

type testUser struct {
	ud     UserDevice
	secret Secret
}

func newTestUser() testUser {
	return testUser{
		ud: UserDevice{
			U: randBytes(6),
			D: randBytes(6),
		},
		secret: GenerateSecret(),
	}
}

func newGameMessageEncoded(t *testing.T, md GameMetadata, b GameMessageBody) GameMessageEncoded {
	ret, err := b.Encode(md)
	require.NoError(t, err)
	return ret
}

func TestCoinflipHappyPath3(t *testing.T)  { happyFollower(t, 3) }
func TestCoinflipHappyPath10(t *testing.T) { happyFollower(t, 10) }

type testBundle struct {
	dh        *testDealersHelper
	dealer    *Dealer
	gameID    GameID
	channelID ChannelID
	leader    testUser
	players   []testUser
	start     Start
}

func (b testBundle) userDevices() []UserDevice {
	var ret []UserDevice
	for _, p := range b.players {
		ret = append(ret, p.ud)
	}
	return ret
}

func (b *testBundle) run(ctx context.Context) {
	go b.dealer.Run(ctx)
}

func setupTestBundle(ctx context.Context, t *testing.T, nUsers int, isLeader bool) *testBundle {
	dh := newTestDealersHelper()
	dealer := NewDealer(dh)

	leader := newTestUser()
	params := NewFlipParametersWithBool()
	start := Start{
		StartTime:            ToTime(dh.clock.Now()),
		CommitmentWindowMsec: 5 * 1000,
		RevealWindowMsec:     5 * 1000,
		SlackMsec:            1 * 1000,
		Params:               params,
	}

	gameID := GenerateGameID()
	channelID := ChannelID(randBytes(6))

	players := []testUser{leader}
	var last testUser
	for i := 0; i < nUsers; i++ {
		tu := newTestUser()
		players = append(players, tu)
		last = tu
	}
	if isLeader {
		dh.me = leader.ud
	} else {
		dh.me = last.ud
	}

	return &testBundle{
		dh:        dh,
		dealer:    dealer,
		gameID:    gameID,
		channelID: channelID,
		leader:    leader,
		players:   players,
		start:     start,
	}
}

func (b *testBundle) md() GameMetadata {
	return GameMetadata{GameID: b.gameID, ChannelID: b.channelID, Initiator: b.leader.ud}
}

func (b *testBundle) commitPhase(ctx context.Context, t *testing.T) {
	cp := CommitmentPayload{
		V: Version_V1,
		U: b.leader.ud.U,
		D: b.leader.ud.D,
		C: b.channelID,
		G: b.gameID,
		S: b.start.StartTime,
	}
	for _, p := range b.players {
		commitment, err := p.secret.computeCommitment(cp)
		require.NoError(t, err)
		body := NewGameMessageBodyWithCommitment(commitment)
		b.dealer.InjectChatMessage(ctx, p.ud, newGameMessageEncoded(t, b.md(), body))
	}
}

func (b *testBundle) startPhase(ctx context.Context, t *testing.T) {
	body := NewGameMessageBodyWithStart(b.start)
	b.dealer.InjectChatMessage(ctx, b.leader.ud, newGameMessageEncoded(t, b.md(), body))
}

func (b *testBundle) completeCommit(ctx context.Context, t *testing.T) {
	b.dealer.InjectChatMessage(ctx, b.leader.ud, newGameMessageEncoded(t, b.md(),
		NewGameMessageBodyWithCommitmentComplete(CommitmentComplete{
			Players: b.userDevices(),
		})))
	update := <-b.dealer.UpdateCh()
	require.NotNil(t, update.CC)
	require.Equal(t, b.userDevices(), update.CC.Players)
}

func (b *testBundle) revealPhase(ctx context.Context, t *testing.T) {
	for _, p := range b.players {
		body := NewGameMessageBodyWithReveal(p.secret)
		b.dealer.InjectChatMessage(ctx, p.ud, newGameMessageEncoded(t, b.md(), body))
	}
	update := <-b.dealer.UpdateCh()
	require.NotNil(t, update.Result)
	require.NotNil(t, update.Result.Bool)
}

func (b *testBundle) stop() {
	b.dealer.Stop()
}

func happyFollower(t *testing.T, nUsers int) {
	ctx := context.Background()
	b := setupTestBundle(ctx, t, nUsers, false)
	b.run(ctx)
	defer b.stop()

	b.startPhase(ctx, t)
	b.commitPhase(ctx, t)
	b.completeCommit(ctx, t)
	b.revealPhase(ctx, t)
}

func TestDroppedReveal(t *testing.T) {
	ctx := context.Background()
	b := setupTestBundle(ctx, t, 3, false)
	b.run(ctx)
	defer b.stop()

	b.startPhase(ctx, t)
	b.commitPhase(ctx, t)
	b.completeCommit(ctx, t)

	for _, p := range b.players[0 : len(b.players)-1] {
		body := NewGameMessageBodyWithReveal(p.secret)
		b.dealer.InjectChatMessage(ctx, p.ud, newGameMessageEncoded(t, b.md(), body))
	}
	b.dh.clock.Advance(time.Duration(13) * time.Second)
	update := <-b.dealer.UpdateCh()
	fmt.Printf("%+v\n", update)
}

func TestLeader(t *testing.T) {
	ctx := context.Background()
	b := setupTestBundle(ctx, t, 3, true)
	b.run(ctx)
	defer b.stop()
	ret, err := b.dealer.StartFlip(ctx, b.start, b.channelID)
	b.gameID = ret.GameID
	require.NoError(t, err)
}
