.PHONY: build test lint install clean release-snapshot vuln

build:
	go build -o bin/straddle ./cmd/straddle

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/straddle

clean:
	rm -rf bin/ dist/

release-snapshot:
	go run github.com/goreleaser/goreleaser/v2@v2.15.4 release --snapshot --clean

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
