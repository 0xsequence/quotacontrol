GO_TEST = $(shell if ! command -v gotest &> /dev/null; then echo "go test"; else echo "gotest"; fi)

.PHONY: build
build:
	go build ./...

.PHONY: proto
proto:
	go generate ./proto

.PHONY: test
test:
	go clean -testcache && $(GO_TEST) -v -p=1 ./...
