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

func TestCoinflipHappyPath(t *testing.T) {
	dh := newTestDealersHelper()
	dealer := NewDealer(dh)
	ctx := context.Background()
	go func() {
		dealer.Run(ctx)
	}()

	leader := newTestUser()
	params := NewFlipParametersWithBool()
	start := Start{
		StartTime:            ToTime(dh.clock.Now()),
		CommitmentWindowMsec: 5 * 1000,
		RevealWindowMsec:     5 * 1000,
		Params:               params,
	}

	gameID := GenerateGameID()
	channelID := ChannelID(randBytes(6))
	md := GameMetadata{GameID: gameID, ChannelID: channelID, Initiator: leader.ud}
	body := NewGameMessageBodyWithStart(start)
	gmwe := newGameMessageWrappedEncoded(t, md, leader.ud, body)

	players := []testUser{leader}
	uds := []UserDevice{leader.ud}
	for i := 0; i < 3; i++ {
		tu := newTestUser()
		players = append(players, tu)
		uds = append(uds, tu.ud)
	}

	dealer.MessageCh() <- gmwe
	cp := CommitmentPayload{
		V: Version_V1,
		U: leader.ud.U,
		D: leader.ud.D,
		C: channelID,
		G: gameID,
		S: start.StartTime,
	}
	for _, p := range players {
		commitment, err := p.secret.computeCommitment(cp)
		require.NoError(t, err)
		body := NewGameMessageBodyWithCommitment(commitment)
		dealer.MessageCh() <- newGameMessageWrappedEncoded(t, md, p.ud, body)
	}
	dealer.MessageCh() <- newGameMessageWrappedEncoded(t, md, leader.ud,
		NewGameMessageBodyWithCommitmentComplete(CommitmentComplete{
			Players: uds,
		}))
	update := <-dealer.UpdateCh()
	require.NotNil(t, update.CC)
	require.Equal(t, uds, update.CC.Players)

	for _, p := range players {
		body := NewGameMessageBodyWithReveal(p.secret)
		dealer.MessageCh() <- newGameMessageWrappedEncoded(t, md, p.ud, body)
	}

	update = <-dealer.UpdateCh()
	require.NotNil(t, update.Result)
	require.NotNil(t, update.Result.Bool)
}
