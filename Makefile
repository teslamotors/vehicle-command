LINTER			= golangci-lint run -v $(LINTER_FLAGS) --exclude-use-default=false --timeout $(LINTER_DEADLINE)
LINTER_DEADLINE	= 30s
LINTER_FLAGS ?=

ifneq (,$(wildcard /etc/alpine-release))
LINTER_FLAGS += --build-tags=musl
endif

all: build

linters:
	@echo "** Running linters...**"
	$(LINTER)
	@echo "** SUCCESS **"

set-version:
	if TAG=$$(git describe --tags --abbrev=0); then echo "$${TAG}" | sed 's/v//' > pkg/account/version.txt; fi

format: set-version
	go fmt ./...

test: install
	go test -cover ./...
	go vet ./...

build: set-version test
	go build ./...

install:
	go install ./cmd/...

doc-images:
	docker run -v ./:/data plantuml/plantuml "doc"

.PHONY: install build linters test format set-version doc-images
