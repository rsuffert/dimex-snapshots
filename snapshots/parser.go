package snapshots

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Parser struct {
	snapshotsByPID    [][]Snapshot // [i][j] = j-th snapshot of i-th process
	invariantCheckers []invariantCheckerFunc
}

func NewParser() (*Parser, error) {
	p := &Parser{
		snapshotsByPID: make([][]Snapshot, len(dumpFiles)),
		invariantCheckers: []invariantCheckerFunc{
			checkMutualExclusion,
			checkWaitingImpliesWantOrInCS,
			checkIdleProcessesState,
		},
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
// For each set of the i-th snapshots of the existing processes, it verifies if the set
// upholds some given invariants. If an invariant violation is detected for the current
// snapshots set, Verify immediately returns an error and stops verifying the subsequent
// invariants for the set. The subsequent sets will also not be verified. It returns nil
// if ALL snapshots of ALL sets uphold ALL the invariants.
func (p *Parser) Verify() error {
	nSnapshots, err := p.getSnapshotsPerPIDCount()
	if err != nil {
		return fmt.Errorf("parser.Verify: %w", err)
	}

	nProcesses := len(p.snapshotsByPID)

	// for each snapshot set (snapID)
	for snapId := 0; snapId < nSnapshots; snapId++ {
		// build the snapshot set with the snapId-th snapshot of each process
		snapshots := make([]Snapshot, nProcesses)
		for pid := 0; pid < nProcesses; pid++ {
			snapshots[pid] = p.snapshotsByPID[pid][snapId]
		}

		// apply the invariants to the snapshot set
		for _, checker := range p.invariantCheckers {
			if err := checker(snapshots...); err != nil {
				return fmt.Errorf("parser.Verify (snap %d): %w", snapId, err)
			}
		}
	}

	return nil
}

// getSnapshotsPerPIDCount checks if all processes (PIDs) have the same number
// of snapshots in the snapshotsByPID map. It returns the count of snapshots
// per PID if they are consistent across all PIDs, or an error if there is a
// mismatch.
//
// Returns:
//   - int: The number of snapshots per PID, if consistent.
//   - error: An error indicating which PID has a mismatch and the expected count
//     if the snapshots are inconsistent. If this is non-nil, then the returned
//     int is undefined and its value should not be used.
func (p *Parser) getSnapshotsPerPIDCount() (int, error) {
	count := len(p.snapshotsByPID[0])
	for pid, snapshots := range p.snapshotsByPID {
		if len(snapshots) != count {
			return 0, fmt.Errorf(
				"parser.getSnapshotsPerPIDCount: process %d has %d snapshots, expected %d as the others",
				pid, len(snapshots), count,
			)
		}
	}
	return count, nil
}
