SHELL := /bin/bash

.PHONY: format format-ui format-go \
	format-check format-check-ui format-check-go \
	lint lint-ui lint-go \
	test test-ui test-go \
	build build-darwin build-ui build-go build-go-darwin

# --- Formatting -------------------------------------------------------------

format: format-ui format-go

format-ui:
	cd front && bun run format

format-go:
	cd back && golangci-lint fmt ./...

format-check: format-check-ui format-check-go

format-check-ui:
	cd front && bun run format:check

format-check-go:
	cd back && golangci-lint fmt --diff ./...

# --- Linting ----------------------------------------------------------------

lint: lint-ui lint-go

lint-ui:
	cd front && bun run lint

lint-go:
	cd back && golangci-lint run ./...

# --- Testing ----------------------------------------------------------------

test: test-ui test-go

test-ui:
	cd front && bun run test

test-go:
	cd back && go test -race -cover ./...

# --- Building ---------------------------------------------------------------

build: build-ui build-go
build-darwin: build-ui build-go-darwin

build-ui:
	cd front && bun run build

build-go:
	cd back && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../api ./cmd/api

build-go-darwin:
	cd back && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../api ./cmd/api
