package snapshots

import (
	"SD/PP2PLink"
	"encoding/json"
	"fmt"
	"os"
)

var dumpFiles = make(map[string]int)

type Snapshot struct {
	ID              int
	PID             int
	State           int
	Waiting         []bool
	LocalClock      int
	ReqTs           int
	NbrResps        int
	InterceptedMsgs []PP2PLink.PP2PLink_Ind_Message

	CollectedResps int `json:"-"`
}

func (s *Snapshot) DumpToFile() error {
	path := fmt.Sprintf("snapshots-pid-%d.txt", s.PID)
	dumpFiles[path] = s.PID

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("snapshot.dumpToFile: failed opening '%s' file: %w", path, err)
	}
	defer file.Close()

	snapshotJson, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("snapshot.dumpToFile: failed marshaling snapshot to JSON: %w", err)
	}

	if _, err = file.WriteString(string(snapshotJson) + "\n"); err != nil {
		return fmt.Errorf("snapshot.dumpToFile: failed writing to '%s' file: %w", path, err)
	}

	return nil
}
