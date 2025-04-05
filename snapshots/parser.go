package snapshots

import (
	"SD/common"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Parser struct {
	snapshotsByPID [][]Snapshot // [i][j] = j-th snapshot of i-th process
}

func NewParser() (*Parser, error) {
	p := &Parser{
		snapshotsByPID: make([][]Snapshot, len(dumpFiles)),
	}

	for dumpFile, pid := range dumpFiles {
		snapshots, err := parseDumpFile(dumpFile)
		if err != nil {
			return nil, fmt.Errorf("snapshots.NewParser parseDumpFile: %w", err)
		}
		p.snapshotsByPID[pid] = snapshots
	}

	return p, nil
}

func parseDumpFile(filename string) ([]Snapshot, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open '%s' file: %w", filename, err)
	}
	defer file.Close()

	var snapshots []Snapshot

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var snapshot Snapshot
		if err := json.Unmarshal(scanner.Bytes(), &snapshot); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func (p *Parser) Verify() error {
	nSnapshots := len(p.snapshotsByPID[0])
	for pid, snapshots := range p.snapshotsByPID {
		if len(snapshots) != nSnapshots {
			return fmt.Errorf("parser.Verify: process %d has %d snapshots, expected %d as the others", pid, len(snapshots), nSnapshots)
		}
	}

	nProcesses := len(p.snapshotsByPID)
	for snapId := 0; snapId < nSnapshots; snapId++ {
		snapshots := make([]Snapshot, nProcesses)
		for pid := 0; pid < nProcesses; pid++ {
			snapshots = append(snapshots, p.snapshotsByPID[pid][snapId])
		}

		// TODO: Call functions for verifying the snapshots that are in the 'snapshots' list
		if err := checkMutualExclusion(snapshots...); err != nil {
			return fmt.Errorf("parser.Verify: %w", err)
		}
		if err := checkWaitingImpliesWantOrInCS(snapshots...); err != nil {
			return fmt.Errorf("parser.Verify: %w", err)
		}
	}

	return nil
}

func checkMutualExclusion(snapshots ...Snapshot) error {
	inCriticalSectionCount := 0
	for _, snapshot := range snapshots {
		if snapshot.State == int(common.InMX) {
			inCriticalSectionCount++
		}
	}

	if inCriticalSectionCount > 1 {
		return fmt.Errorf("checkMutualExclusion: %d processes in critical section (more than 1)", inCriticalSectionCount)
	}

	return nil
}

func checkWaitingImpliesWantOrInCS(snapshots ...Snapshot) error {
	for _, snapshot := range snapshots {
		if !common.Any(snapshot.Waiting, func(w bool) bool { return w }) {
			continue
		}
		if snapshot.State != int(common.InMX) && snapshot.State != int(common.WantMX) {
			return fmt.Errorf("checkWaitingImpliesWantOrInCS: process %d is delaying responses but not InMX or WantMX", snapshot.PID)
		}
	}

	return nil
}
