package common

type State int // enumeracao dos estados possiveis de um processo
const (
	NoMX State = iota
	WantMX
	InMX
)
