CONFIG_FILE:="etc/config.json"
VERSION:=latest
PWD:=$(shell pwd)

build:
	@go build -o $(PWD)/bin/server ./server/server.go

server-start: build
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server start

server-status:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server status || true

server-stop:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server stop

server-kill:
	@CONFIG_FILE=$(CONFIG_FILE) ./scripts/helper.sh server kill

client-run:
	@CONFIG_FILE=$(CONFIG_FILE) VERSION=$(VERSION) go run ./client/$(VERSION)/client.go