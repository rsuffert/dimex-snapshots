package DIMEX

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	SNAP string = "snap"
)

type snapshot struct {
	ID         int
	PID        int
	State      State
	Waiting    []bool
	LocalClock int
	ReqTs      int
	NbrResps   int

	collectedResps int
}

func (s snapshot) String() string {
	bytes, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Errorf("snapshot.String: failed marshalling snapshot: %w", err))
	}
	return string(bytes)
}

func (s *snapshot) DumpToFile(path string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Errorf("snapshot.dumpToFile: failed opening '%s' file: %w", path, err))
	}
	defer file.Close()

	_, err = file.WriteString(s.String() + "\n")
	if err != nil {
		panic(fmt.Errorf("snapshot.dumpToFile: failed writing to '%s' file: %w", path, err))
	}
}
