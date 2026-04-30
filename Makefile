LINTER			= golangci-lint run -v $(LINTER_FLAGS) --exclude-use-default=false --timeout $(LINTER_DEADLINE)
LINTER_DEADLINE	= 30s
LINTER_FLAGS ?=

ifneq (,$(wildcard /etc/alpine-release))
LINTER_FLAGS += --build-tags=musl
endif

PROTO_DIR=./pkg/protocol/protobuf
PROTO_FILES=$(wildcard $(PROTO_DIR)/*.proto)

# Pinned protoc toolchain. Update both versions here AND in Dockerfile.protoc
# when bumping; expect a diff in the regenerated *.pb.go files.
# PROTOC_VERSION uses the upstream release tag (e.g. "21.9"); the matching
# runtime version reported by `protoc --version` is "3.21.9".
PROTOC_VERSION=21.9
PROTOC_GEN_GO_VERSION=1.28.1
PROTOC_IMAGE=vehicle-command-protoc:$(PROTOC_VERSION)-$(PROTOC_GEN_GO_VERSION)
PROTOC_DOCKERFILE=Dockerfile.protoc

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

proto-gen: $(PROTO_FILES)
	protoc --proto_path $(PROTO_DIR) --go_out $(PROTO_DIR) --go_opt=module=github.com/teslamotors/vehicle-command/pkg/protocol/protobuf ${PROTO_FILES}

# Build the pinned protoc toolchain image. No-op if it already exists.
proto-builder:
	docker build \
	    --build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
	    --build-arg PROTOC_GEN_GO_VERSION=$(PROTOC_GEN_GO_VERSION) \
	    -f $(PROTOC_DOCKERFILE) \
	    -t $(PROTOC_IMAGE) \
	    .

# Regenerate *.pb.go using the pinned toolchain. Use this instead of
# `proto-gen` to get reproducible output independent of the host's
# installed protoc version.
proto-gen-docker: proto-builder
	docker run --rm \
	    -v "$(CURDIR):/workspace" \
	    -w /workspace \
	    $(PROTOC_IMAGE) \
	    --proto_path $(PROTO_DIR) \
	    --go_out $(PROTO_DIR) \
	    --go_opt=module=github.com/teslamotors/vehicle-command/pkg/protocol/protobuf \
	    $(PROTO_FILES)

# Regenerate via Docker and fail if the result differs from what is
# committed. Intended for CI to catch out-of-date *.pb.go files.
proto-check: proto-gen-docker
	@if ! git diff --quiet -- $(PROTO_DIR); then \
	    echo "ERROR: generated protobuf files are out of date."; \
	    echo "Run 'make proto-gen-docker' and commit the result."; \
	    git --no-pager diff -- $(PROTO_DIR); \
	    exit 1; \
	fi
	@echo "Generated protobuf files are up to date."

.PHONY: install build linters test format set-version doc-images proto-gen proto-builder proto-gen-docker proto-check
