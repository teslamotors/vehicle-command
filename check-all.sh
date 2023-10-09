#!/bin/bash

set -e

go build ./...
go test ./...
go vet ./...
FILES_FORMATTED=$(go fmt ./...)
if [[ $(echo "$FILES_FORMATTED" | wc -l)  -gt 1 ]]; then
    echo "The following files were reformatted by go fmt:"
    echo "$FILES_FORMATTED"
    exit 1
fi

if command -v shellcheck > /dev/null; then
    find . -name "*.sh" -exec shellcheck {} \;
fi
