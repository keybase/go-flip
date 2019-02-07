package flip

import (
	"context"
	"fmt"
	clockwork "github.com/keybase/clockwork"
	// "github.com/stretchr/testify/require"
	// "math/big"
	// "testing"
	"time"
)

type chatServer struct {
	shutdownCh  chan struct{}
	inputCh     chan GameMessageWrappedEncoded
	chatClients []*chatClient
}

type chatClient struct {
	clock  clockwork.FakeClock
	me     UserDevice
	ch     chan GameMessageWrappedEncoded
	server *chatServer
}

func (t *chatClient) Clock() clockwork.Clock {
	return t.clock
}

func (t *chatClient) ServerTime(context.Context) (time.Time, error) {
	return t.clock.Now(), nil
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

func (t *chatServer) run() {
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

func newChatServer() *chatServer {
	return &chatServer{
		shutdownCh: make(chan struct{}),
		inputCh:    make(chan GameMessageWrappedEncoded),
	}
}

func (s *chatServer) newClient() *chatClient {
	ret := &chatClient{
		clock:  clockwork.NewFakeClock(),
		me:     newTestUser(),
		ch:     make(chan GameMessageWrappedEncoded),
		server: s,
	}
	s.chatClients = append(s.chatClients, ret)
	return ret
}
