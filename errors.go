package flip

import (
	"fmt"
	"strings"
)

type Error struct {
	NoCommitments  []Player
	NoReveals      []Player
	BadCommitments []Player
}

func (e *Error) addNoCommitment(u Player) {
	e.NoCommitments = append(e.NoCommitments, u)
}

func (e *Error) addNoReveal(u Player) {
	e.NoReveals = append(e.NoReveals, u)
}

func (e *Error) addBadCommitment(u Player) {
	e.BadCommitments = append(e.BadCommitments, u)
}

func (e Error) IsNil() bool {
	return len(e.NoCommitments)+len(e.NoReveals)+len(e.BadCommitments) == 0
}

func (e Error) format(out []string, what string, users []Player) []string {
	if len(users) == 0 {
		return out
	}
	var userStrings []string
	for _, u := range users {
		userStrings = append(userStrings, string(u))
	}
	p := fmt.Sprintf("Players %s: %s", what, strings.Join(userStrings, ","))
	return append(out, p)
}

func (e Error) Error() string {
	var parts []string
	parts = e.format(parts, "without commitments", e.NoCommitments)
	parts = e.format(parts, "without reveals", e.NoReveals)
	parts = e.format(parts, "with bad commitments", e.BadCommitments)
	return fmt.Sprintf("Errors in flip: %s", strings.Join(parts, ";"))
}
