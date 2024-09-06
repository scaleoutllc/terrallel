export CGO_ENABLED = 0

all: build

install-tools:
	@go mod download
	@echo Installing tools from tools.go
	@cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %

fmt:
	go fmt ./...

vet:
	go vet ./... && staticcheck ./...

ifeq ($(OS),Windows_NT)
    DEV_NULL = NUL
else
    DEV_NULL = /dev/null
endif
test:
	go generate ./...
	go test -v ./... -coverprofile=$(DEV_NULL)

validate: fmt lint vet test

build: validate
	go build -o ./dist/terrallel main.go

run: validate
	go run ./

.PHONY: all fmt lint vet test validate build run install-tools