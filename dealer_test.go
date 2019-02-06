package flip

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	clockwork "github.com/keybase/clockwork"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testDealersHelper struct {
	clock clockwork.FakeClock
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
	return newTestUser().ud
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

func newGameMessageWrappedEncoded(t *testing.T, md GameMetadata, sender UserDevice, b GameMessageBody) GameMessageWrappedEncoded {
	v1 := GameMessageV1{
		Md:   md,
		Body: b,
	}
	msg := NewGameMessageWithV1(v1)
	raw, err := msgpackEncode(msg)
	require.NoError(t, err)
	return GameMessageWrappedEncoded{
		Sender: sender,
		Body:   base64.StdEncoding.EncodeToString(raw),
	}
}

func TestCoinflipHappyPath3(t *testing.T)  { happyTester(t, 3) }
func TestCoinflipHappyPath10(t *testing.T) { happyTester(t, 10) }

type testBundle struct {
	dh        *testDealersHelper
	dealer    *Dealer
	gameID    GameID
	channelID ChannelID
	md        GameMetadata
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

func setupTestBundle(ctx context.Context, t *testing.T, nUsers int) *testBundle {
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
	md := GameMetadata{GameID: gameID, ChannelID: channelID, Initiator: leader.ud}

	players := []testUser{leader}
	for i := 0; i < nUsers; i++ {
		tu := newTestUser()
		players = append(players, tu)
	}
	return &testBundle{
		dh:        dh,
		dealer:    dealer,
		gameID:    gameID,
		channelID: channelID,
		md:        md,
		leader:    leader,
		players:   players,
		start:     start,
	}
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
		b.dealer.MessageCh() <- newGameMessageWrappedEncoded(t, b.md, p.ud, body)
	}
}

func (b *testBundle) startPhase(ctx context.Context, t *testing.T) {
	body := NewGameMessageBodyWithStart(b.start)
	gmwe := newGameMessageWrappedEncoded(t, b.md, b.leader.ud, body)
	b.dealer.MessageCh() <- gmwe
}

func (b *testBundle) completeCommit(ctx context.Context, t *testing.T) {
	b.dealer.MessageCh() <- newGameMessageWrappedEncoded(t, b.md, b.leader.ud,
		NewGameMessageBodyWithCommitmentComplete(CommitmentComplete{
			Players: b.userDevices(),
		}))
	update := <-b.dealer.UpdateCh()
	require.NotNil(t, update.CC)
	require.Equal(t, b.userDevices(), update.CC.Players)
}

func (b *testBundle) revealPhase(ctx context.Context, t *testing.T) {
	for _, p := range b.players {
		body := NewGameMessageBodyWithReveal(p.secret)
		b.dealer.MessageCh() <- newGameMessageWrappedEncoded(t, b.md, p.ud, body)
	}
	update := <-b.dealer.UpdateCh()
	require.NotNil(t, update.Result)
	require.NotNil(t, update.Result.Bool)
}

func (b *testBundle) stop() {
	b.dealer.Stop()
}

func happyTester(t *testing.T, nUsers int) {
	ctx := context.Background()
	b := setupTestBundle(ctx, t, nUsers)
	b.run(ctx)
	b.startPhase(ctx, t)
	b.commitPhase(ctx, t)
	b.completeCommit(ctx, t)
	b.revealPhase(ctx, t)
	b.stop()
}
