.PHONY: build
build:
	go build ./...

.PHONY: proto
proto:
	go generate ./proto

.PHONY: test
test:
	go clean -testcache && go test -v -p=1 ./...
