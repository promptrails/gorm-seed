.PHONY: all build test lint fmt vet coverage clean check bench \
        coverage-html example update-deps install-hooks changelog

all: fmt vet lint test build

## Build all packages
build:
	go build ./...

## Run all tests
test:
	go test -race -count=1 ./...

## Run tests with coverage
coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

## Open HTML coverage report
coverage-html: coverage.out
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html

## Run linters
lint:
	golangci-lint run ./...

## Format code
fmt:
	gofmt -w -s .
	goimports -w .

## Run go vet
vet:
	go vet ./...

## Clean build artifacts
clean:
	rm -f coverage.out coverage.html

## Run benchmarks
bench:
	go test -bench=. -benchmem ./...

## Run all checks (pre-commit)
check: fmt vet lint test build

## Run the basic example
example:
	go run ./examples/basic/

## Update Go module dependencies
update-deps:
	go get -u ./...
	go mod tidy

## Generate changelog from git tags
changelog:
	@echo "# Changelog" > CHANGELOG.md
	@echo "" >> CHANGELOG.md
	@git log --oneline --decorate --no-merges --format="* %s (%h)" | head -50 >> CHANGELOG.md
