.PHONY: build test lint install clean

build:
	go build -o bin/straddle-pp-cli ./cmd/straddle-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/straddle-pp-cli

clean:
	rm -rf bin/
