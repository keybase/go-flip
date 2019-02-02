package flip

import (
	"fmt"
	"strings"
)

type Error struct {
	NoCommitments  []User
	NoReveals      []User
	BadCommitments []User
}

func (e *Error) addNoCommitment(u User) {
	e.NoCommitments = append(e.NoCommitments, u)
}

func (e *Error) addNoReveal(u User) {
	e.NoReveals = append(e.NoReveals, u)
}

func (e *Error) addBadCommitment(u User) {
	e.BadCommitments = append(e.BadCommitments, u)
}

func (e Error) IsNil() bool {
	return len(e.NoCommitments)+len(e.NoReveals)+len(e.BadCommitments) == 0
}

func (e Error) format(out []string, what string, users []User) []string {
	if len(users) == 0 {
		return out
	}
	var userStrings []string
	for _, u := range users {
		userStrings = append(userStrings, string(u))
	}
	p := fmt.Sprintf("Users %s: %s", what, strings.Join(userStrings, ","))
	return append(out, p)
}

func (e Error) Error() string {
	var parts []string
	parts = e.format(parts, "without commitments", e.NoCommitments)
	parts = e.format(parts, "without reveals", e.NoReveals)
	parts = e.format(parts, "with bad commitments", e.BadCommitments)
	return fmt.Sprintf("Errors in flip: %s", strings.Join(parts, ";"))
}
