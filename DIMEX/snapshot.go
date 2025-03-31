package DIMEX

import (
	"SD/PP2PLink"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	SNAP string = "snap"
)

type snapshot struct {
	ID              int
	PID             int
	State           State
	Waiting         []bool
	LocalClock      int
	ReqTs           int
	NbrResps        int
	InterceptedMsgs []PP2PLink.PP2PLink_Ind_Message

	collectedResps int
}

func (s *snapshot) DumpToFile() error {
	path := fmt.Sprintf("snapshots-pid-%d.txt", s.PID)

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

func (m *DIMEX_Module) handleIncomingSnap(msg PP2PLink.PP2PLink_Ind_Message) {
	parts := strings.Split(msg.Message, ";")
	snapId, _ := strconv.Atoi(parts[1])

	takeSnapshot := m.lastSnapshot == nil || m.lastSnapshot.ID < snapId
	if takeSnapshot {
		logrus.Debugf("\t\tP%d: taking snapshot %d\n", m.id, snapId)
		m.takeSnapshot(snapId)
	}

	m.lastSnapshot.collectedResps++

	snapshotOver := m.lastSnapshot.collectedResps == (len(m.addresses) - 1)
	if snapshotOver {
		logrus.Debugf("\t\tP%d: snapshot %d completed. Dumping to file...\n", m.id, snapId)
		if err := m.lastSnapshot.DumpToFile(); err != nil {
			logrus.Errorf("P%d: error dumping snapshot %d to file: %v\n", m.id, snapId, err)
		}
	}
}

func (m *DIMEX_Module) startSnapshot() {
	snapId := 0
	if m.lastSnapshot != nil {
		snapId = m.lastSnapshot.ID + 1
	}
	m.takeSnapshot(snapId)
}

func (m *DIMEX_Module) takeSnapshot(snapId int) {
	waiting := make([]bool, len(m.waiting))
	copy(waiting, m.waiting)
	m.lastSnapshot = &snapshot{
		ID:              snapId,
		PID:             m.id,
		State:           m.st,
		Waiting:         waiting,
		LocalClock:      m.lcl,
		ReqTs:           m.reqTs,
		NbrResps:        m.nbrResps,
		InterceptedMsgs: make([]PP2PLink.PP2PLink_Ind_Message, 0),
	}

	for i, addr := range m.addresses {
		if i != m.id {
			m.sendToLink(
				addr,
				fmt.Sprintf("%s;%d", SNAP, snapId),
				fmt.Sprintf("PID %d", m.id),
			)
			logrus.Debugf("P%d: sent SNAP to %s\n", m.id, addr)
		}
	}
}
