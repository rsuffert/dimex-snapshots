package common

// State represents all possible states for a process in a ditributed mutual
// exclusion (DiMEX) context.
type State int

const (
	NoMX State = iota
	WantMX
	InMX
)
