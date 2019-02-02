package flip

type TimeoutError struct {
	missing []string
}

type DuplicateCommitmentError struct {
	u string
}

type DuplicateHashError struct {
	u string
}
