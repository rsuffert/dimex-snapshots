# DiMEx Snapshots

## Overview

This project has been developed as the first assignment for the Distributed System course at PUCRS. It consists in a system that runs *n* processes that communicate through messages to syncrhonize the access to a shared file (`mxOUT.txt`), which is treated as a critical section (CS). Periodically, one of the processes will take the initiative of capturing a snapshot of the system, recording its state in a file (`snapshots-pid-<n>.txt`) and sending messages to the other processes to do the same. In the end, when the program exits, the snapshots in the files will be checked against inconsistencies.

## Usage

The program has a `Makefile` for easier execution. To run the application with the default arguments (verbosity logging enabled, failure simulation disabled, and 3 processes), simply run `make` at the root of the project. The default configurations can be overwritten with:

```bash
make ARGS="[-v] [-f] <ip-address:port> <ip-address:port> [<ip-address:port>...]" 
```

Each `<ip-address:port>` pair is the address of a process of the system. The `-v` flag is optionally supplied for enabling verbose logging. The `-f` flag is optionally supplied for enabling failure simulation in the DiMEx module and trigger snapshot invariant violations.

**NOTE**: To avoid retaining snapshots from previous executions in the files, prefer running the program using `make`, as the default recipe will run `make clean` to remove those output files and only then run `make run`.

## Structure

```
.
├── common
│   ├── slices.go            # common operations with slices (not part of stdlib)
│   └── states.go            # the possible states of a process in the access to the CS
├── dimex
│   └── dimex.go             # distributed mutual exclusion implementation (and snapshots)
├── go.mod
├── go.sum
├── main.go                  # entrypoint for the application
├── Makefile
├── pp2plink
│   └── pp2plink.go          # implementation of a perfect point to point link for the processes to communicate
├── README.md
└── snapshots
    ├── invariants.go        # implementation of snapshot invariants to be checked
    ├── parser.go            # implementation of snapshot files parsing logic
    └── snapshots.go         # implementation of snapshot files generation logic
```

## Acknowledgements

The skeleton for this code (first commit of the repository) has been provided by **Prof. Fernando Luis Dotti (PUCRS)**.