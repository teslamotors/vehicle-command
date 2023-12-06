all: build

format:
	git describe --tags --abbrev=0 | sed 's/v//' > pkg/account/version.txt
	go fmt ./...

test: 
	go test -cover ./...
	go vet ./...

build: format test
	go build ./...

install: format test
	go install ./cmd/...

.PHONY: install build test format
