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

func checkPlayer(err *Error, user PlayerState) {
	if user.Commitment.IsNil() {
		err.addNoCommitment(user.Player)
		return
	}
	if user.Reveal.IsNil() {
		err.addNoReveal(user.Player)
		return
	}

	if !checkReveal(user.Commitment, user.Reveal) {
		err.addBadCommitment(user.Player)
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

func computeSecret(users []PlayerState) Secret {
	var res Secret
	for _, u := range users {
		res.XOR(u.Commitment)
	}
	return res
}

func Flip(users []PlayerState) (*PRNG, error) {
	err := checkPlayers(users)
	if err != nil {
		return nil, err
	}
	res := computeSecret(users)
	return NewPRNG(res), nil
}

func FlipOneBig(users []PlayerState, modulus *big.Int) (*big.Int, error) {
	prng, err := Flip(users)
	if err != nil {
		return nil, err
	}
	return prng.Big(modulus), nil
}

func FlipInt(users []PlayerState, modulus int64) (int64, error) {
	prng, err := Flip(users)
	if err != nil {
		return 0, err
	}
	return prng.Int(modulus), nil
}

func FlipBool(users []PlayerState) (bool, error) {
	prng, err := Flip(users)
	if err != nil {
		return false, err
	}
	return prng.Bool(), nil
}
