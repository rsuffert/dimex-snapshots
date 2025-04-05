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

// Verify checks the consistency and correctness of the snapshots stored in the Parser.
// If any of validation fails, an error is returned immediately with details about the
// first detected failure. If all validations pass, nil is returned.
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

// checkMutualExclusion verifies that the mutual exclusion property is upheld
// across the provided snapshots. It ensures that at most one process is in
// the critical section at any given time.
//
// Parameters:
//
//	snapshots - A variadic list of Snapshot objects representing the state
//	            of different processes.
//
// Returns:
//
//	An error if more than one process is found to be in the critical section,
//	otherwise nil.
func checkMutualExclusion(snapshots ...Snapshot) error {
	inCSCount := common.Count(snapshots, func(s Snapshot) bool {
		return s.State == int(common.InMX)
	})
	if inCSCount > 1 {
		return fmt.Errorf("checkMutualExclusion: %d processes in critical section (more than 1)", inCSCount)
	}
	return nil
}

// checkWaitingImpliesWantOrInCS verifies that for each snapshot provided, if the process
// is in a "waiting" state (indicated by any `true` value in the `Waiting` slice),
// then the process must either be in the "InMX" state (critical section) or the "WantMX" state
// (intending to enter the critical section). If this condition is violated, an error is returned.
//
// Parameters:
//
//	snapshots - A variadic parameter of Snapshot objects representing the state of processes.
//
// Returns:
//
//	error - An error describing the violation if the condition is not met, or nil if all snapshots
//	        satisfy the condition.
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
