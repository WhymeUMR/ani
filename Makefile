.PHONY: build run test tidy

build:
	go build -o bin/ani ./cmd/ani

run:
	go run ./cmd/ani

test:
	go test ./...

tidy:
	go mod tidy
