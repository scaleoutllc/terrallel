export CGO_ENABLED = 0

all: build

fmt:
	go fmt ./...

vet:
	go vet ./... && staticcheck ./...

test:
	go test -v ./... -coverprofile /dev/null

build: fmt lint vet test
	go build ./...

run: build
	go run ./

.PHONY: all fmt lint vet test build run