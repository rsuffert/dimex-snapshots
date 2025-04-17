package main

import (
	"SD/DIMEX"
	"SD/snapshots"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	setLogger()

	if len(flag.Args()) < 2 {
		logrus.Errorf("Usage: %s [-v] <address:port> <address:port> [<address:port>...]", os.Args[0])
		os.Exit(1)
	}

	addresses := flag.Args()

	for i := range addresses {
		dmx := DIMEX.NewDIMEX(addresses, i, false)
		go worker(dmx)
	}

	terminate()
}

// worker simulates the flow of an application that uses the DIMEX module
// this code was provided as part of the skeleton implementation of the DIMEX module
// and was slightly modified to work with the new implementation
func worker(dmx *DIMEX.DIMEX_Module) {
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
		dmx.Req <- DIMEX.ENTER

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
		dmx.Req <- DIMEX.EXIT
	}
}

func setLogger() {
	verboseMode := flag.Bool("v", false, "Enable verbose (debug) logging for snapshots")
	flag.Parse()

	loggingLevel := logrus.InfoLevel
	if *verboseMode {
		loggingLevel = logrus.DebugLevel
	}

	logrus.SetLevel(loggingLevel)
}

func terminate() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan // blocks until one of the signals above is received

	logrus.Infof("Received '%s' signal. Exiting...\n\n", sig)

	logrus.Infof("Instantiating a parser to verify the snapshots...")
	snapsParser, err := snapshots.NewParser()
	if err != nil {
		logrus.Errorf("Failed to instantiate parser: %v", err)
		os.Exit(1)
	}

	logrus.Infof("Parsing and verifying snapshots...")
	if err := snapsParser.ParseVerify(); err != nil {
		logrus.Infof("Inconsistency detected in snapshots: %v", err)
		os.Exit(1)
	}
	logrus.Infof("No inconsistencies detected in snapshots!")
}
