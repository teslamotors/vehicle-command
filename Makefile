all: build

format:
	git describe --tags --abbrev=0 | sed 's/v//' > pkg/account/version.txt
	go fmt ./...
.PHONY: format

test: format
	go test ./...
	go vet ./...
.PHONY: test

build: test
	go build ./...
.PHONY: build

install: test
	go install ./cmd/...
.PHONY: install
