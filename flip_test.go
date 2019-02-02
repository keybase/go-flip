package flip

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func makeTestSecret(b byte) Secret {
	var ret Secret
	ret[1] = 0xee
	ret[0] = b
	return ret
}

func makeTestPlayer(b byte) PlayerState {
	s := makeTestSecret(b)
	return PlayerState{
		Player:     Player(fmt.Sprintf("u%d", b)),
		Commitment: s.Hash(),
		Reveal:     s,
	}
}

func TestFlip(t *testing.T) {
	var users []PlayerState
	for i := 1; i < 20; i++ {
		users = append(users, makeTestPlayer(byte(i)))
	}
	i, err := FlipInt(users, int64(10033))
	require.NoError(t, err)
	require.Equal(t, i, int64(5412))
}
