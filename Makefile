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

test:
	go test -v ./... -coverprofile /dev/null

validate: fmt lint vet test

build: validate
	go build -o ./dist ./...

run: validate
	go run ./

.PHONY: all fmt lint vet test validate build run install-tools