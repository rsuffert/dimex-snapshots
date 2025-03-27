SRC := main.go
ARGS ?= 127.0.0.1:5000 127.0.0.1:6001 127.0.0.1:7002
CLEAN_FILES := *.txt

.PHONY: all clean run

all: clean run

clean:
	rm -rf $(CLEAN_FILES)

run:
	go run $(SRC) $(ARGS)