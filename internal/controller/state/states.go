package state

type State = bool

// This is a bool and not using iota enum style
// for now, because we only have these two states right meow.
// The purpose is to indicate whether the controller should return or continue.
// This is necessary, because in the non-error branch (case) it's unclear how to continue.
const (
	CONTINUE State = true
	RETURN   State = false
)
