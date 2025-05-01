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

// ProcessState is a struct that represents the application-specific state of a process
// for the snapshot.
type ProcessState struct {
	ID         int
	PID        int
	State      common.State
	Waiting    []bool
	LocalClock int
	ReqTs      int
	NbrResps   int
}

// communicationChan is a struct that represents the abstraction of a communication
// channel between two processes. It contains a slice of all messages sent through it
// and a boolean indicating whether the channel is open or closed for sending new messages.
type communicationChan struct {
	messages []pp2plink.IndMsg
	isOpen   bool
}

// AddMessage records the message as received through this communication channel, as
// long as it's open. If not, the message is discarded and not recorded.
func (c *communicationChan) AddMessage(msg pp2plink.IndMsg) {
	if !c.isOpen {
		return
	}
	c.messages = append(c.messages, msg)
}

// Close closes the communication channel, indicating that no more messages can be stored
// as received through it.
func (c *communicationChan) Close() {
	c.isOpen = false
}

// MarshalJSON customizes the JSON representation of the CommunicationChan struct.
func (c *communicationChan) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Messages []pp2plink.IndMsg `json:"Messages"`
		IsOpen   bool              `json:"IsOpen"`
	}
	return json.Marshal(&Alias{
		Messages: c.messages,
		IsOpen:   c.isOpen,
	})
}

// Snapshot is a struct that represents a snapshot of a process in the system.
type Snapshot struct {
	ID         int
	PID        int
	State      common.State
	Waiting    []bool
	LocalClock int
	ReqTs      int
	NbrResps   int

	// Communication chans between this process and the process with the PID in the key
	// Used for storing messages in transit when this snapshot was taken
	CommunicationChans map[int]*communicationChan
}

// NewSnapshot creates a new Snapshot instance.
func NewSnapshot(state ProcessState) *Snapshot {
	nProcesses := len(state.Waiting)

	s := &Snapshot{
		ID:                 state.ID,
		PID:                state.PID,
		State:              state.State,
		Waiting:            state.Waiting,
		LocalClock:         state.LocalClock,
		ReqTs:              state.ReqTs,
		NbrResps:           state.NbrResps,
		CommunicationChans: make(map[int]*communicationChan, nProcesses),
	}

	for i := 0; i < nProcesses; i++ {
		s.CommunicationChans[i] = &communicationChan{
			messages: make([]pp2plink.IndMsg, 0),
			isOpen:   true,
		}
	}
	s.CommunicationChans[s.PID].Close() // the channel to myself is always closed

	return s
}

// DumpToFile appends the dump of the snapshot to a file in JSON format.
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

// HasMessagesInTransit checks if there are any messages in transit in the snapshot.
func (s *Snapshot) HasMessagesInTransit() bool {
	for _, commChan := range s.CommunicationChans {
		if len(commChan.messages) > 0 {
			return true
		}
	}
	return false
}

// IsOver tells whether or not this snapshot is over (completed) and therefore can
// be dumped to a file.
func (s *Snapshot) IsOver() bool {
	for _, commChan := range s.CommunicationChans {
		if commChan.isOpen {
			return false
		}
	}
	return true
}
