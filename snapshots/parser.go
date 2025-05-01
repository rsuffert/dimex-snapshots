package snapshots

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type parser struct {
	files             []*os.File
	scanners          []*bufio.Scanner
	invariantCheckers []invariantCheckerFunc
}

// NewParser creates a new instance of a Parser, which can then be used for verifying the snapshots
// taken by the processes during the execution of the mutual exclusion algorithm. The user is
// responsible for calling Init() before performing any operations on the Parser instance, and
// Close() after all operations are completed to ensure proper resource management.
func NewParser() *parser {
	nProcesses := len(dumpFiles)
	p := &parser{
		files:    make([]*os.File, nProcesses),
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

	return p
}

// Init initializes the Parser instance.
func (p *parser) Init() error {
	dumpFilesMu.RLock()
	defer dumpFilesMu.RUnlock()

	for dumpFile, pid := range dumpFiles {
		file, err := os.Open(dumpFile)
		if err != nil {
			return fmt.Errorf("parser.Init: failed to open file '%s': %w", dumpFile, err)
		}
		p.files[pid] = file
		p.scanners[pid] = bufio.NewScanner(file)
	}
	return nil
}

// Close terminates the Parser instance and releases any resources it holds.
func (p *parser) Close() error {
	for _, file := range p.files {
		if err := file.Close(); err != nil {
			return fmt.Errorf("parser.Close: failed to close file: %w", err)
		}
	}
	return nil
}

// ParseVerify iterates through all snapshot files, parses and validates them using
// the registered invariant checkers. It ensures that each snapshot set adheres to
// the defined invariants.
//
// If an error occurs while reading the snapshots or if an invariant violation
// is detected, the method immediately aborts and returns an error with detailed
// context, including the snapshot ID where the issue occurred.
//
// Returns nil if all snapshots are successfully verified without errors.
func (p *parser) ParseVerify() error {
	snapId := 0

	for {
		snapshots, err := p.getNextSnapshotsSet()
		if err != nil {
			return fmt.Errorf("parser.ParseVerify (snapId %d): error reading snapshots: %w", snapId, err)
		}

		if snapshots == nil {
			// no more snapshots to verify
			break
		}

		for _, checker := range p.invariantCheckers {
			if err := checker(*snapshots...); err != nil {
				return fmt.Errorf("parser.ParseVerify (snapId %d): invariant violation: %w", snapId, err)
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
func (p *parser) getNextSnapshotsSet() (*[]Snapshot, error) {
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

	if eofCount == 0 {
		// all scanners have provided a snapshot
		return &snapshots, nil
	}

	if eofCount != nProcesses {
		// only some scanners have reached EOF (other scanners still have snapshots)
		return nil, fmt.Errorf(
			"parser.getNextSnapshotsSet: %d processes have reached EOF, but %d processes still have snapshots remaining",
			eofCount,
			nProcesses-eofCount,
		)
	}

	// all scanners have reached EOF - no more snapshots to read
	return nil, nil
}
