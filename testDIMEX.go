package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	outFilePath string = "mxOUT.txt"
)

func main() {
	if err := checkOutFile(outFilePath); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("File is correct!")
}

func checkOutFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	var expectedChar byte = '|'
	nReadChars := 0
	for {
		nReadChars++
		readChar, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error reading byte from file: %v", err)
		}

		if readChar != expectedChar {
			return fmt.Errorf("unexpected character read from file: %c (char n° %d)", readChar, nReadChars)
		}

		if expectedChar == '|' {
			expectedChar = '.'
		} else {
			expectedChar = '|'
		}
	}

	return nil
}
