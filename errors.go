package flip

import (
	"fmt"
	"strings"
)

type Error struct {
	NoCommitments  []Player
	NoReveals      []Player
	BadCommitments []Player
	Duplicates     []Player
}

func (e *Error) addNoCommitment(p Player) {
	e.NoCommitments = append(e.NoCommitments, p)
}

func (e *Error) addNoReveal(p Player) {
	e.NoReveals = append(e.NoReveals, p)
}

func (e *Error) addBadCommitment(p Player) {
	e.BadCommitments = append(e.BadCommitments, p)
}

func (e *Error) addDuplicate(p Player) {
	e.Duplicates = append(e.Duplicates, p)
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
	parts = e.format(parts, "with duplicated IDs", e.Duplicates)
	return fmt.Sprintf("Errors in flip: %s", strings.Join(parts, ";"))
}
