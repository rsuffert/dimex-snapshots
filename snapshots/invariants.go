package snapshots

import (
	"SD/common"
	"fmt"
)

// invariantCheckerFunc is a function type that checks invariants on a set of snapshots.
type invariantCheckerFunc func(snapshots ...Snapshot) error

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
		return s.State == common.InMX
	})
	if inCSCount > 1 {
		return fmt.Errorf("checkMutualExclusion: %d processes in critical section (more than 1)", inCSCount)
	}
	return nil
}

// checkWaitingImpliesWantOrInCS verifies that for each snapshot provided, if the process
// is delaying entry responses to other processes, then it must either be in the "InMX"
// state (critical section) or the "WantMX" state (intending to enter the critical section).
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
		isDelayingResps := common.Any(snapshot.Waiting, func(w bool) bool { return w })
		if !isDelayingResps {
			continue
		}
		if snapshot.State != common.InMX && snapshot.State != common.WantMX {
			return fmt.Errorf("checkWaitingImpliesWantOrInCS: process %d is delaying responses but not InMX or WantMX", snapshot.PID)
		}
	}
	return nil
}

// checkIdleProcessesState verifies the state of a set of snapshots to ensure that, if all processes are idle,
// then no process is delaying entry responses or has intercepted messages.
//
// Parameters:
//
//	snapshots - A variadic parameter representing a list of Snapshot objects to be checked.
//
// Returns:
//
//	error - Returns an error if any process is delaying responses or has intercepted messages
//	        when all processes are idle. Returns nil otherwise.
func checkIdleProcessesState(snapshots ...Snapshot) error {
	allIdle := common.All(snapshots, func(s Snapshot) bool {
		return s.State == common.NoMX
	})
	if !allIdle {
		return nil
	}

	for _, snapshot := range snapshots {
		isDelayingResps := common.Any(snapshot.Waiting, func(w bool) bool { return w })
		if isDelayingResps {
			return fmt.Errorf("checkIdleProcessesState: process %d is delaying responses, but all processes are idle", snapshot.PID)
		}
		if len(snapshot.InterceptedMsgs) > 0 {
			return fmt.Errorf("checkIdleProcessesState: process %d has intercepted messages, but all processes are idle", snapshot.PID)
		}
	}
	return nil
}

// checkOnlyInMXWithAllConsent verifies that a process in the critical section has received the
// responses from all other processes to the entry request.
//
// Parameters:
//
//	snapshots - A variadic parameter representing a list of Snapshot objects to be checked.
//
// Returns:
//
//	error - Returns an error if a process is in the critical section but has not received
//	        responses from all other processes. Returns nil if all processes in the critical section
//	        have received the responses from all other processes.
func checkOnlyInMXWithAllConsent(snapshots ...Snapshot) error {
	nProcesses := len(snapshots)

	for _, snapshot := range snapshots {
		isInMX := snapshot.State == common.InMX
		allCollectedResps := snapshot.NbrResps == (nProcesses - 1) // don't count itself
		if isInMX && !allCollectedResps {
			return fmt.Errorf(
				"checkOnlyInMXWithAllConsent: process %d is in MX but not all responses received (only %d)",
				snapshot.PID,
				snapshot.NbrResps,
			)
		}
	}

	return nil
}

// checkNotOtherDelaysWhenInMX verifies that if a process is in the critical section, no other
// process is delaying the entry response to it.
//
// Parameters:
//
//	snapshots - A variadic parameter representing a list of Snapshot objects to be checked.
//
// Returns:
//
//	error - Returns an error if a process is in the critical section and another process is
//	        delaying the entry response. Returns nil if no such violations are found.
func checkNotOtherDelaysWhenInMX(snapshots ...Snapshot) error {
	for _, snapshot := range snapshots {
		if snapshot.State != common.InMX {
			continue
		}

		for _, othSnapshot := range snapshots {
			if othSnapshot.Waiting[snapshot.PID] {
				return fmt.Errorf(
					"checkNotOtherDelaysWhenInMX: process %d is in MX but process %d is delaying the entry response",
					snapshot.PID,
					othSnapshot.PID,
				)
			}
		}
	}

	return nil
}

// checkNotDelayingWhenNoMX verifies that, if a process is in the NoMX state, it is not delaying
// entry responses to other processes.
//
// Parameters:
//
//	snapshots - A variadic parameter representing a list of Snapshot objects to be checked.
//
// Returns:
//
//	error - Returns an error if a process is in the NoMX state but is delaying entry responses.
//	        Returns nil if no such violations are found.
func checkNotDelayingWhenNoMX(snapshots ...Snapshot) error {
	for _, snapshot := range snapshots {
		if snapshot.State != common.NoMX {
			continue
		}

		isDelayingResps := common.Any(snapshot.Waiting, func(w bool) bool { return w })
		if isDelayingResps {
			return fmt.Errorf(
				"checkNotDelayingWhenNoMX: process %d is NoMX but is delaying responses",
				snapshot.PID,
			)
		}
	}

	return nil
}
