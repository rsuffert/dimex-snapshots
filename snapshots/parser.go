package snapshots

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Parser struct {
	scanners          []*bufio.Scanner
	invariantCheckers []invariantCheckerFunc
}

// NewParser creates a new instance of a Parser, which can then be used for verifying the snapshots
// taken by the processes during the execution of the mutual exclusion algorithm.
func NewParser() (*Parser, error) {
	nProcesses := len(dumpFiles)
	p := &Parser{
		scanners: make([]*bufio.Scanner, nProcesses),
		invariantCheckers: []invariantCheckerFunc{
			checkMutualExclusion,
			checkWaitingImpliesWantOrInCS,
			checkIdleProcessesState,
			checkOnlyInMXWithAllConsent,
			checkNotOtherDelaysWhenInMX,
			checkNotDelayingWhenNoMX,
		},
	}

	for dumpFile, pid := range dumpFiles {
		file, err := os.Open(dumpFile)
		if err != nil {
			return nil, fmt.Errorf("snapshots.NewParser failed to open file '%s': %w", dumpFile, err)
		}
		p.scanners[pid] = bufio.NewScanner(file)
	}

	return p, nil
}

// Verify iterates through all snapshot sets and validates them using the
// registered invariant checkers. It ensures that each snapshot set adheres
// to the defined invariants.
//
// If an error occurs while reading the snapshots or if an invariant violation
// is detected, the method returns an error with detailed context, including
// the snapshot ID where the issue occurred.
//
// Returns nil if all snapshots are successfully verified without errors.
func (p *Parser) Verify() error {
	snapId := 0

	for {
		snapshots, err := p.getNextSnapshotsSet()
		if err != nil {
			return fmt.Errorf("parser.Verify (snapId %d): error reading snapshots: %w", snapId, err)
		}

		if snapshots == nil {
			// no more snapshots to verify
			break
		}

		for _, checker := range p.invariantCheckers {
			if err := checker(*snapshots...); err != nil {
				return fmt.Errorf("parser.Verify (snapId %d): invariant violation: %w", snapId, err)
			}
		}

		snapId++
	}

	return nil
}

// getNextSnapshotsSet reads the next set of snapshots from all scanners in the Parser.
// It expects each scanner to provide a JSON-encoded snapshot, which is unmarshaled into
// a Snapshot struct. The function ensures that all scanners have the same number of snapshots.
//
// Returns:
//   - A pointer to a slice of Snapshot structs if all scanners successfully provide a snapshot.
//   - An error if any scanner encounters an issue (e.g., read error, unmarshaling error, or mismatched snapshot counts).
//   - A nil pointer and no error if all scanners have reached EOF and there are no more snapshots to read.
func (p *Parser) getNextSnapshotsSet() (*[]Snapshot, error) {
	nProcesses := len(p.scanners)
	snapshots := make([]Snapshot, nProcesses)
	eofCount := 0

	for pid, scanner := range p.scanners {
		if !scanner.Scan() {
			if scanner.Err() != nil {
				return nil, fmt.Errorf("parser.getNextSnapshotsSet (pid %d): error reading line: %w", pid, scanner.Err())
			}
			// this scanner has reached EOF
			eofCount++
			continue
		}

		var s Snapshot
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			return nil, fmt.Errorf("parser.getNextSnapshotsSet (pid %d): error unmarshaling snapshot: %w", pid, err)
		}
		snapshots[pid] = s
	}

	if eofCount > 0 {
		if eofCount != nProcesses {
			return nil, fmt.Errorf(
				"parser.getNextSnapshotsSet: %d processes have reached EOF, but %d processes still have snapshots remaining",
				eofCount,
				nProcesses-eofCount,
			)
		}
		return nil, nil // all scanners reached EOF
	}

	return &snapshots, nil
}
