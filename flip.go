package flip

import ()

type User string

type UserState struct {
	User       User
	Commitment *Secret
	Reveal     *Secret
}
