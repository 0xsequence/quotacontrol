CONFIG_FILE := etc/config.json
PWD := $(shell pwd)

build:
	@cd server && GOWORK=off go build -o $(PWD)/bin/server .

server-start: build
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server start

server-status:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server status || true

server-stop:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server stop

server-kill:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server kill

client-run: client-run-latest client-run-v0.23

common-test:
	@cd common && GOWORK=off go test ./...

client-run-latest:
	@cd client/latest && CONFIG_FILE=../../$(CONFIG_FILE) GOWORK=off go run .

client-run-v0.23:
	@cd client/v0.23 && CONFIG_FILE=../../$(CONFIG_FILE) GOWORK=off go run .
