.PHONY: all build test lint fmt vet coverage clean

all: fmt vet lint test build

## Build all packages
build:
	go build ./...

## Run all tests
test:
	go test -race -count=1 ./...

## Run tests with coverage
coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## Run linters
lint:
	golangci-lint run ./...

## Format code
fmt:
	gofmt -w .
	goimports -w .

## Run go vet
vet:
	go vet ./...

## Clean build artifacts
clean:
	rm -f coverage.out coverage.html
