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

build: fmt lint vet test
	go build -o ./dist ./...

run: fmt lint vet test
	go run ./

.PHONY: all fmt lint vet test build run install-tools