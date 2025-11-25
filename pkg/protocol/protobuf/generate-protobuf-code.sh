#!/usr/bin/env bash

protoc --proto_path . --go_out . --go_opt=module=github.com/teslamotors/vehicle-command/pkg/protocol/protobuf ./*.proto
