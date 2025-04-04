package snapshots

import (
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
	// TODO: add logic for verifying the snapshots and
	// returning an error if an inconsistency is detected
	return nil
}
