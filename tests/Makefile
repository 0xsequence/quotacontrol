CONFIG_FILE := etc/config.json
PWD := $(shell pwd)

build:
	@GOWORK=off go build -o $(PWD)/bin/server ./server/server.go

server-start: build
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server start

server-status:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server status || true

server-stop:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server stop

server-kill:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server kill

client-run: client-run-latest client-run-v0.23

client-run-latest:
	@CONFIG_FILE=$(CONFIG_FILE) GOWORK=off go run ./client/latest/client.go

client-run-v0.23:
	@CONFIG_FILE=$(CONFIG_FILE) GOWORK=off go run ./client/v0.23/client.go
