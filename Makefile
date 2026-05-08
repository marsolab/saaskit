SHELL := /bin/bash

.PHONY: format format-ui format-go lint lint-ui lint-go test test-ui test-go build build-ui build-go

format: format-ui format-go

format-ui:
	cd ui && bun run format

format-go:
	golangci-lint fmt -d ./...

lint: lint-ui lint-go

lint-ui:
	cd ui && bun run lint

lint-go:
	golangci-lint run ./...

test: test-ui test-go

test-ui:
	cd ui && bun run test

test-go:
	go test -race -cover ./...

build: build-ui build-go
build-darwin: build-ui build-go-darwin

build-ui:
	cd ui && bun run build

build-go:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cpage ./cmd

build-go-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o cpage ./cmd
