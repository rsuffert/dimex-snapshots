package main

import (
	"SD/DIMEX"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: %s <address:port> <address:port> [<address:port>...]", os.Args[0])
	}

	addresses := os.Args[1:]

	for i := range addresses {
		dmx := DIMEX.NewDIMEX(addresses, i, false)
		go worker(dmx)
	}

	// wait forever
	select {}
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
