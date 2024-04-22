all: build

set-version:
	if TAG=$$(git describe --tags --abbrev=0); then echo "$${TAG}" | sed 's/v//' > pkg/account/version.txt; fi

format: set-version
	go fmt ./...

test: 
	go test -cover ./...
	go vet ./...

build: set-version test
	go build ./...

install: test
	go install ./cmd/...

doc-images:
	docker run -v ./:/data plantuml/plantuml "doc"

.PHONY: install build test format set-version doc-images
