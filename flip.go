package flip

import (
	"math/big"
	"strings"
)

// Player is an identifier for a player in the flip game. It can be anything,
// but must be unique for the game.  You can use user IDs in hex here, for instance.
type Player string

func (p Player) key() string {
	return strings.ToLower(string(p))
}

// PlayerState refers to all state about a player in the game. It includes the
// Player's name, his commitment, and his revealed preimage.
type PlayerState struct {
	Player     Player
	Commitment Secret
	Reveal     Secret
}

func checkReveal(c Secret, r Secret) bool {
	return r.Hash().Eq(c)
}

func checkPlayer(err *Error, player PlayerState) {
	if player.Commitment.IsNil() {
		err.addNoCommitment(player.Player)
		return
	}
	if player.Reveal.IsNil() {
		err.addNoReveal(player.Player)
		return
	}

	if !checkReveal(player.Commitment, player.Reveal) {
		err.addBadCommitment(player.Player)
		return
	}

	return
}

func checkPlayers(player []PlayerState) error {
	var err Error
	d := make(map[string]bool)
	for _, p := range player {
		checkPlayer(&err, p)
		if d[p.Player.key()] {
			err.addDuplicate(p.Player)
		} else {
			d[p.Player.key()] = true
		}
	}
	if err.IsNil() {
		return nil
	}

	return err
}

func computeSecret(players []PlayerState) Secret {
	var res Secret
	for _, p := range players {
		res.XOR(p.Commitment)
	}
	return res
}

// Flip takes all the completed PlayerStates from the game, makes sure they don't have
// an error, and if not, outputs a PRNG from which arbirarily many ints, or bools,
// can be deterministically plucked. If there's an error, PRNG will be nil.
func Flip(players []PlayerState) (*PRNG, error) {
	err := checkPlayers(players)
	if err != nil {
		return nil, err
	}
	res := computeSecret(players)
	return NewPRNG(res), nil
}

// FlipOneBig takes all the completed PlayerStates, and checks them.  If no error,
// then outputs one random number between 0 and the given modulus, which is an arbitrarily
// big number. If there was an error in the game setup, then it will return nil and the error.
func FlipOneBig(players []PlayerState, modulus *big.Int) (*big.Int, error) {
	prng, err := Flip(players)
	if err != nil {
		return nil, err
	}
	return prng.Big(modulus), nil
}

// FlipOneInt takes all the completed PlayerStates, and checks them.  If no error,
// then outputs one random number between 0 and the given modulus, a signed 64-bit int.
// If there was an  error in the game setup, then it will return 0 and the error.
func FlipInt(players []PlayerState, modulus int64) (int64, error) {
	prng, err := Flip(players)
	if err != nil {
		return 0, err
	}
	return prng.Int(modulus), nil
}

// FlipOneBool takes all the completed PlayerStates, and checks them. If no error,
// then outputs one random bool. If there was an error in the game setup, then it will
// return false and the error.
func FlipBool(players []PlayerState) (bool, error) {
	prng, err := Flip(players)
	if err != nil {
		return false, err
	}
	return prng.Bool(), nil
}
