TEST_FLAGS ?= -p 8 -failfast -race -shuffle on
GOTOOLCHAIN := $(shell cat go.mod | grep "^go" | tr -d ' ')

all:
	@echo "make <cmd>:"
	@echo ""
	@echo "commands:"
	@awk -F'[ :]' '/^#+/ {comment=$$0; gsub(/^#+[ ]*/, "", comment)} !/^(_|all:)/ && /^([A-Za-z_-]+):/ && !seen[$$1]++ {printf "  %-24s %s\n", $$1, (comment ? "- " comment : ""); comment=""} !/^#+/ {comment=""}' Makefile

test-clean:
	go clean -testcache

test: test-clean
	go test -timeout 10s -run=$(TEST) $(TEST_FLAGS) -json ./... | go run github.com/mfridman/tparse --all --follow

test-rerun: test-clean
	go run github.com/goware/rerun/cmd/rerun -watch ./ -run 'make test'

test-coverage:
	go test -run=$(TEST) $(TEST_FLAGS) -cover -coverprofile=coverage.out -json ./... | tparse --all --follow

test-coverage-inspect: test-coverage
	go tool cover -html=coverage.out

generate:
	WEBRPC_SCHEMA_VERSION=$(shell git log -1 --date=format:'v0-%y.%-m.%-d' --format='%ad+%h' ./proto/*.ridl) \
	GOTOOLCHAIN=$(GOTOOLCHAIN) go generate -x ./...

proto: generate

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run ./... --fix -c .golangci.yml
