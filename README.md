# DiMEx Snapshots

## Overview

This project has been developed as the first assignment for the Distributed System course at PUCRS. It consists in a system that runs *n* processes that communicate through messages to syncrhonize the access to a shared file (`mxOUT.txt`), which is treated as a critical section (CS). Periodically, one of the processes will take the initiative of capturing a snapshot of the system, recording its state in a file (`snapshots-pid-<n>.txt`) and sending messages to the other processes to do the same. In the end, when the program exits, the snapshots in the files will be checked against inconsistencies.

## Usage

### Using `make` to run the program

The program has a `Makefile` for easier execution. To run the application with the default command (using the pre-defined command-line arguments), **simply run `make` at the root of the project**. The pre-defined command-line arguments can be overwritten with:

```bash
make ARGS="[-v] [-f] [-s <seconds>] <ip-address:port> <ip-address:port> [<ip-address:port>...]" 
```

Each `<ip-address:port>` pair is the address of a process of the system.

**NOTE**: To avoid retaining snapshots from previous executions in the files, prefer running the program using `make`, as the default recipe will run `make clean` to remove those output files and only then run `make run`.

### Optional flags

The command-line flags in the below table are supported to customize the execution.

| Flag           | Type      | Meaning                                               | Default value (if flag not specified) |
|--------------- |-----------|-------------------------------------------------------|---------------------------------------|
| `-v`           | `bool`    | Enable verbose logging                                | False                                 |
| `-f`           | `bool`    | Enable failure simulation in the DiMEx module         | False                                 |
| `-s <seconds>` | `float64` | Interval in which snapshots will be taken, in seconds | 0.5                                   |

**NOTE**: When setting your interval for taking snapshots, keep in mind that the system only supports one snapshot being taken at a time. Therefore, your interval should be large enough so that all processes that you're using have time to take their snapshots, flood the snapshot message, and dump their snapshots to their file when the responses are received.

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