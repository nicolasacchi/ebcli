VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
MODULE := github.com/nicolasacchi/ebcli
BINARY := ebcli

.PHONY: build test lint clean release install

build:
	go build $(LDFLAGS) -o $(BINARY) .

install:
	go install $(LDFLAGS) .

test:
	go test ./...

test-v:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -f $(BINARY)
	rm -rf dist/

release: clean
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .
	@echo "Built binaries in dist/"
	@ls -lh dist/

.DEFAULT_GOAL := build
