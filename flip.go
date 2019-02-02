package flip

import (
	"math/big"
)

type Player string

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
	d := make(map[Player]bool)
	for _, p := range player {
		checkPlayer(&err, p)
		if d[p.Player] {
			err.addDuplicate(p.Player)
		} else {
			d[p.Player] = true
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

func Flip(players []PlayerState) (*PRNG, error) {
	err := checkPlayers(players)
	if err != nil {
		return nil, err
	}
	res := computeSecret(players)
	return NewPRNG(res), nil
}

func FlipOneBig(players []PlayerState, modulus *big.Int) (*big.Int, error) {
	prng, err := Flip(players)
	if err != nil {
		return nil, err
	}
	return prng.Big(modulus), nil
}

func FlipInt(players []PlayerState, modulus int64) (int64, error) {
	prng, err := Flip(players)
	if err != nil {
		return 0, err
	}
	return prng.Int(modulus), nil
}

func FlipBool(players []PlayerState) (bool, error) {
	prng, err := Flip(players)
	if err != nil {
		return false, err
	}
	return prng.Bool(), nil
}
