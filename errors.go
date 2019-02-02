package flip

type TimeoutError struct {
	missing []User
}

type DuplicateCommitmentError struct {
	u User
}

type DuplicateHashError struct {
	u User
}

type BadRevealError struct {
	u User
}
