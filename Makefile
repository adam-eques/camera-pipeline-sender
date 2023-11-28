MODULE = $(shell go list -m)
APP_NAME = camera-pipeline-sender

.PHONY: generate build test lint

generate:
	go generate ./...

build: # build a server
	go build -a -o $(APP_NAME) $(MODULE)/cmd

test:
	go clean -testcache
	go test ./... -v

run:
	go run $(MODULE)/cmd