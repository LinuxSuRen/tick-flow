BIN ?= bin/tick-flow

.PHONY: build test lint run clean

build:
	go build -o $(BIN) ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run

run:
	go run ./cmd/server

clean:
	rm -rf bin
