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

generate-mocks:
	mockgen -source=pkg/proxy/proxy.go -destination mocks/proxy.go -package=mocks -mock_names Vehicle=ProxyVehicle,Account=ProxyAccount

.PHONY: install build test format set-version doc-images generate-mocks
