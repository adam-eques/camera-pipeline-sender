MODULE = $(shell go list -m)
APP_NAME = camera-pipeline-sender

.PHONY: generate build test lint

generate:
	go generate ./...

build: # build a server
	set PKG_CONFIG_PATH="./tools/x264"
	go build -a -o $(APP_NAME) $(MODULE)/cmd

test:
	set PKG_CONFIG_PATH="./tools/x264"
	go clean -testcache
	go test ./... -v

run:
	set PKG_CONFIG_PATH="./tools/x264"
	go run $(MODULE)/cmd