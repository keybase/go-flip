package flip

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	clockwork "github.com/keybase/clockwork"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testDealersHelper struct {
	clock clockwork.FakeClock
	me    UserDevice
	ch    chan GameMessageEncoded
}

func newTestDealersHelper(me UserDevice) *testDealersHelper {
	return &testDealersHelper{
		clock: clockwork.NewFakeClock(),
		me:    me,
		ch:    make(chan GameMessageEncoded),
	}
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

func (t *testDealersHelper) SendChat(ctx context.Context, chid ChannelID, msg GameMessageEncoded) error {
	fmt.Printf("Sending chat %s <- %s\n", hex.EncodeToString(chid), msg)
	go func() {
		t.ch <- msg
	}()
	return nil
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

func newTestUser() UserDevice {
	return UserDevice{
		U: randBytes(6),
		D: randBytes(6),
	}
}

func newGameMessageEncoded(t *testing.T, md GameMetadata, b GameMessageBody) GameMessageEncoded {
	ret, err := b.Encode(md)
	require.NoError(t, err)
	return ret
}

type testBundle struct {
	me        UserDevice
	dh        *testDealersHelper
	dealer    *Dealer
	channelID ChannelID
	start     Start
	leader    *PlayerControl
	followers []*PlayerControl
}

func (b *testBundle) run(ctx context.Context) {
	go b.dealer.Run(ctx)
}

func setupTestBundle(ctx context.Context, t *testing.T) *testBundle {
	me := newTestUser()
	dh := newTestDealersHelper(me)
	dealer := NewDealer(dh)

	params := NewFlipParametersWithBool()
	start := Start{
		StartTime:            ToTime(dh.clock.Now()),
		CommitmentWindowMsec: 5 * 1000,
		RevealWindowMsec:     5 * 1000,
		SlackMsec:            1 * 1000,
		Params:               params,
	}
	channelID := ChannelID(randBytes(6))

	return &testBundle{
		me:        me,
		dh:        dh,
		dealer:    dealer,
		channelID: channelID,
		start:     start,
	}
}

func (b *testBundle) makeFollowers(t *testing.T, n int) {
	for i := 0; i < n; i++ {
		b.makeFollower(t)
	}
}

func (b *testBundle) runFollowersCommit(t *testing.T) {
	for _, f := range b.followers {
		b.sendCommitment(t, f)
	}
}

func (b *testBundle) sendCommitment(t *testing.T, p *PlayerControl) {
	p.sendCommitment()
	b.receiveCommitmentFrom(t, p)
}

func (b *testBundle) receiveCommitmentFrom(t *testing.T, p *PlayerControl) {
	res := <-b.dealer.UpdateCh()
	require.NotNil(t, res.Commitment)
	require.Equal(t, p.me, *res.Commitment)
}

func (b *testBundle) makeFollower(t *testing.T) {
	f, err := b.dealer.newPlayerControl(newTestUser(), b.leader.GameMetadata(), b.start)
	require.NoError(t, err)
	b.followers = append(b.followers, f)
}

func (b *testBundle) stop() {
	b.dealer.Stop()
}

func TestLeader(t *testing.T) {
	ctx := context.Background()
	b := setupTestBundle(ctx, t)
	b.run(ctx)
	defer b.stop()
	leader, err := b.dealer.StartFlip(ctx, b.start, b.channelID)
	b.leader = leader
	require.NoError(t, err)
	<-b.dh.ch
	b.receiveCommitmentFrom(t, leader)
	<-b.dh.ch
	b.makeFollowers(t, 4)
	b.runFollowersCommit(t)
	b.dh.clock.Advance(time.Duration(6001) * time.Millisecond)
	msg := <-b.dealer.UpdateCh()
	require.NotNil(t, msg.CommitmentComplete)
	require.Equal(t, 5, len(msg.CommitmentComplete.Players))
	<-b.dh.ch
}
