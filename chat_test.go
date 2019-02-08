package flip

import (
	"context"
	"fmt"
	clockwork "github.com/keybase/clockwork"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
	"time"
)

type chatServer struct {
	shutdownCh  chan struct{}
	inputCh     chan GameMessageWrappedEncoded
	chatClients []*chatClient
	clock       clockwork.FakeClock
}

type chatClient struct {
	shutdownCh chan struct{}
	me         UserDevice
	ch         chan GameMessageWrappedEncoded
	server     *chatServer
	dealer     *Dealer
}

func (t *chatClient) Clock() clockwork.Clock {
	return t.server.clock
}

func (t *chatClient) ServerTime(context.Context) (time.Time, error) {
	return t.Clock().Now(), nil
}

func (t *chatClient) CLogf(ctx context.Context, fmtString string, args ...interface{}) {
	fmt.Printf(fmtString+"\n", args...)
}

func (t *chatClient) ReadHistory(ctx context.Context, since time.Time) ([]GameMessageWrappedEncoded, error) {
	return nil, nil
}

func (t *chatClient) Me() UserDevice {
	return t.me
}

func (t *chatClient) SendChat(ctx context.Context, chid ChannelID, msg GameMessageEncoded) error {
	t.server.inputCh <- GameMessageWrappedEncoded{Body: msg, Sender: t.me}
	return nil
}

func (t *chatServer) run(ctx context.Context) {
	for {
		select {
		case <-t.shutdownCh:
			return
		case msg := <-t.inputCh:
			for _, cli := range t.chatClients {
				if !cli.me.Eq(msg.Sender) {
					cli.ch <- msg
				}
			}
		}
	}
}

func (t *chatServer) stop() {
	close(t.shutdownCh)
}

func newChatServer() *chatServer {
	return &chatServer{
		clock:      clockwork.NewFakeClock(),
		shutdownCh: make(chan struct{}),
		inputCh:    make(chan GameMessageWrappedEncoded, 1000),
	}
}

func (s *chatServer) newClient() *chatClient {
	ret := &chatClient{
		shutdownCh: make(chan struct{}),
		me:         newTestUser(),
		ch:         make(chan GameMessageWrappedEncoded, 1000),
		server:     s,
	}
	ret.dealer = NewDealer(ret)
	s.chatClients = append(s.chatClients, ret)
	return ret
}

func (c *chatClient) run(ctx context.Context) {
	go c.dealer.Run(ctx)
	for {
		select {
		case <-c.shutdownCh:
			return
		case msg := <-c.ch:
			c.dealer.InjectIncomingChat(ctx, msg.Sender, msg.Body)
		}
	}
}

func (s *chatServer) makeAndRunClients(ctx context.Context, nClients int) []*chatClient {
	for i := 0; i < nClients; i++ {
		cli := s.newClient()
		go cli.run(ctx)
	}
	return s.chatClients
}

func forAllClients(clients []*chatClient, f func(c *chatClient)) {
	for _, cli := range clients {
		f(cli)
	}
}

func nTimes(n int, f func()) {
	for i := 0; i < n; i++ {
		f()
	}
}

func (c *chatClient) consumeCommitment(t *testing.T) {
	msg := <-c.dealer.UpdateCh()
	require.NotNil(t, msg.Commitment)
}

func (c *chatClient) consumeCommitmentComplete(t *testing.T, n int) {
	msg := <-c.dealer.UpdateCh()
	require.NotNil(t, msg.CommitmentComplete)
	require.Equal(t, n, len(msg.CommitmentComplete.Players))
}

func (c *chatClient) consumeReveal(t *testing.T) {
	msg := <-c.dealer.UpdateCh()
	require.NotNil(t, msg.Reveal)
}

func (c *chatClient) consumeAbsteneesError(t *testing.T, n int) {
	msg := <-c.dealer.UpdateCh()
	require.Error(t, msg.Err)
	ae, ok := msg.Err.(AbsenteesError)
	require.True(t, ok)
	require.Equal(t, n, len(ae.Absentees))
}

func (c *chatClient) consumeResult(t *testing.T, r **big.Int) {
	msg := <-c.dealer.UpdateCh()
	require.NotNil(t, msg.Result)
	require.NotNil(t, msg.Result.Big)
	if *r == nil {
		*r = msg.Result.Big
	}
	require.Equal(t, 0, msg.Result.Big.Cmp(*r))
}

func (c *chatClient) stop() {
	close(c.shutdownCh)
}

func (c *chatServer) stopClients() {
	for _, cli := range c.chatClients {
		cli.stop()
	}
}

func TestHappyChat10(t *testing.T) {
	testHappyChat(t, 10)
}

func TestHappyChat100(t *testing.T) {
	testHappyChat(t, 100)
}

func testHappyChat(t *testing.T, n int) {
	srv := newChatServer()
	ctx := context.Background()
	go srv.run(ctx)
	defer srv.stop()
	clients := srv.makeAndRunClients(ctx, n)
	defer srv.stopClients()

	start := NewStartWithBigInt(srv.clock.Now(), pi())
	channelID := ChannelID(randBytes(6))
	_, err := clients[0].dealer.StartFlip(ctx, start, channelID)
	require.NoError(t, err)
	forAllClients(clients, func(c *chatClient) { nTimes(n, func() { c.consumeCommitment(t) }) })
	srv.clock.Advance(time.Duration(4001) * time.Millisecond)
	forAllClients(clients, func(c *chatClient) { c.consumeCommitmentComplete(t, n) })
	forAllClients(clients, func(c *chatClient) { nTimes(n, func() { c.consumeReveal(t) }) })
	var b *big.Int
	forAllClients(clients, func(c *chatClient) { c.consumeResult(t, &b) })
}

func TestSadChatOneAbsentee(t *testing.T) {
	testSadAbsentees(t, 10, 1)
}

func TestSadChatFiveAbsentee(t *testing.T) {
	testSadAbsentees(t, 20, 5)
}

func testSadAbsentees(t *testing.T, nTotal int, nAbstentees int) {
	srv := newChatServer()
	ctx := context.Background()
	go srv.run(ctx)
	defer srv.stop()
	clients := srv.makeAndRunClients(ctx, nTotal)
	defer srv.stopClients()

	start := NewStartWithBigInt(srv.clock.Now(), pi())
	channelID := ChannelID(randBytes(6))
	_, err := clients[0].dealer.StartFlip(ctx, start, channelID)
	require.NoError(t, err)
	present := nTotal - nAbstentees
	forAllClients(clients, func(c *chatClient) { nTimes(nTotal, func() { c.consumeCommitment(t) }) })
	forAllClients(clients[present:], func(c *chatClient) { c.dealer.Stop() })
	clients = clients[0:present]
	srv.clock.Advance(time.Duration(4001) * time.Millisecond)
	forAllClients(clients, func(c *chatClient) { c.consumeCommitmentComplete(t, nTotal) })
	forAllClients(clients, func(c *chatClient) { nTimes(present, func() { c.consumeReveal(t) }) })
	srv.clock.Advance(time.Duration(31001) * time.Millisecond)
	forAllClients(clients, func(c *chatClient) { c.consumeAbsteneesError(t, nAbstentees) })
}
