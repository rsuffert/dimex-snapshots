package snapshots

import (
	"encoding/json"
	"fmt"
	"os"
	"pucrs/sd/common"
	"pucrs/sd/pp2plink"
	"sync"
)

var (
	dumpFiles   = make(map[string]int)
	dumpFilesMu sync.RWMutex
)

type Snapshot struct {
	ID              int
	PID             int
	State           common.State
	Waiting         []bool
	LocalClock      int
	ReqTs           int
	NbrResps        int
	InterceptedMsgs []pp2plink.IndMsg

	CollectedResps int `json:"-"`
}

func (s *Snapshot) DumpToFile() error {
	path := fmt.Sprintf("snapshots-pid-%d.txt", s.PID)

	dumpFilesMu.Lock()
	dumpFiles[path] = s.PID // store the name of the file and the PID of the process for later parsing
	dumpFilesMu.Unlock()

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
