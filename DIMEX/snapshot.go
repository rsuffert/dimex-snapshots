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

func (s *snapshot) DumpToFile() {
	path := fmt.Sprintf("snapshots-pid-%d.txt", s.PID)

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

func (m *DIMEX_Module) handleIncomingSnap(msg PP2PLink.PP2PLink_Ind_Message) {
	parts := strings.Split(msg.Message, ";")
	initiatorId, _ := strconv.Atoi(parts[1])
	snapId, _ := strconv.Atoi(parts[2])

	takeSnapshot := m.lastSnapshot == nil || m.lastSnapshot.ID < snapId
	if takeSnapshot {
		logrus.Debugf("\t\tP%d: taking snapshot %d\n", m.id, snapId)
		m.takeSnapshot(snapId, initiatorId)
	}

	m.lastSnapshot.collectedResps++

	snapshotOver := m.lastSnapshot.collectedResps == (len(m.addresses) - 1)
	if snapshotOver {
		logrus.Debugf("\t\tP%d: snapshot %d completed. Dumping to file...\n", m.id, snapId)
		m.lastSnapshot.DumpToFile()
	}
}

func (m *DIMEX_Module) startSnapshot() {
	snapId := 0
	if m.lastSnapshot != nil {
		snapId = m.lastSnapshot.ID + 1
	}
	m.takeSnapshot(snapId, m.id)
}

func (m *DIMEX_Module) takeSnapshot(snapId, initiatorId int) {
	waiting := make([]bool, len(m.waiting))
	copy(waiting, m.waiting)
	m.lastSnapshot = &snapshot{
		ID:         snapId,
		PID:        m.id,
		State:      m.st,
		Waiting:    waiting,
		LocalClock: m.lcl,
		ReqTs:      m.reqTs,
		NbrResps:   m.nbrResps,
	}

	for i, addr := range m.addresses {
		if i != m.id {
			m.sendToLink(
				addr,
				fmt.Sprintf("%s;%d;%d", SNAP, initiatorId, snapId),
				fmt.Sprintf("PID %d", m.id),
			)
			logrus.Debugf("P%d: sent SNAP to %s\n", m.id, addr)
		}
	}
}
