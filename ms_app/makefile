PROJECT_NAME = ms_app
OS = linux
ARCH = amd64
BUILD_FROM = ./cmd/${PROJECT_NAME}
BUILD_TO = ./app/${PROJECT_NAME}

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'


init:
	go mod init ${PROJECT_NAME} && go mod tidy

## build: build project
.PHONY: build
build:
	GOOS=${OS} GOARCH=${ARCH} CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-w -s" -o ${BUILD_TO} ${BUILD_FROM}

## migration-up: up the migration stage with the database
.PHONY: migration UP
migration-up:
	go run cmd/migration/main.go -action up

## migration-down: DOWN the migration stage with the database
.PHONY: migration DOWN
migration-down:
	go run cmd/migration/main.go -action down

## fast-start: start project
.PHONY: build
fast-start:
	go run cmd/$(PROJECT_NAME)/main.go
