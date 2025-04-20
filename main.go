package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"pucrs/sd/dimex"
	"pucrs/sd/snapshots"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	BOLD_GREEN = "\033[1;32m"
	BOLD_RED   = "\033[1;31m"
	RESET      = "\033[0m"
)

func main() {
	verboseMode := flag.Bool("v", false, "Enable verbose (debug) logging for snapshots")
	failureMode := flag.Bool("f", false, "Enable failure simulation in the DiMEx module")
	flag.Parse()

	if len(flag.Args()) < 2 {
		logrus.Errorf("Usage: %s [-v] [-f] <address:port> <address:port> [<address:port>...]", os.Args[0])
		os.Exit(1)
	}

	loggingLevel := logrus.InfoLevel
	if *verboseMode {
		loggingLevel = logrus.DebugLevel
	}
	logrus.SetLevel(loggingLevel)

	dimexOpts := make([]dimex.Opt, 0)
	if *failureMode {
		logrus.Debug("Enabling failure simulation in the DiMEx module")
		dimexOpts = append(dimexOpts, dimex.WithFailOpt())
	}

	addresses := flag.Args()
	for i := range addresses {
		dmx := dimex.NewDimex(
			addresses,
			i,
			dimexOpts...,
		)
		go worker(dmx)
	}

	terminate()
}

// worker simulates the flow of an application that uses the DIMEX module
// this code was provided as part of the skeleton implementation of the DIMEX module
// and was slightly modified to work with the new implementation
func worker(dmx *dimex.Dimex) {
	// open file that all processes should write to
	file, err := os.OpenFile("./mxOUT.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// wait for a few seconds so all processes can be initialized
	time.Sleep(2 * time.Second)

	// do application-specific work to simulate the use of the DIMEX module
	for {
		// asks to access the DIMEX
		dmx.Req <- dimex.ENTER

		// waits for the DIMEX to be released by other processes
		<-dmx.Ind

		// write entry to file
		if _, err := file.WriteString("|"); err != nil {
			fmt.Println("Error writing to file:", err)
			return
		}

		// write exit to file
		if _, err := file.WriteString("."); err != nil {
			fmt.Println("Error writing to file:", err)
			return
		}

		// release the DIMEX module
		dmx.Req <- dimex.EXIT
	}
}

func terminate() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan // blocks until one of the signals above is received

	logrus.Infof("Received '%s' signal. Executing termination routine...\n", sig)

	logrus.Infof("Instantiating a parser to verify the snapshots...")
	snapsParser := snapshots.NewParser()
	if err := snapsParser.Init(); err != nil {
		logrus.Errorf("Failed to Init parser: %v", err)
		os.Exit(1)
	}
	defer snapsParser.Close()

	logrus.Infof("Parsing and verifying snapshots...")
	if err := snapsParser.ParseVerify(); err != nil {
		logrus.Infof("%sInconsistency detected in snapshots: %v%s\n", BOLD_RED, err, RESET)
		os.Exit(1)
	}
	logrus.Infof("%sNo inconsistencies detected in snapshots!%s\n", BOLD_GREEN, RESET)
}
