.PHONY: run once dry test build

run:
	go run ./cmd/jobhunter

once:
	go run ./cmd/jobhunter --once

dry:
	go run ./cmd/jobhunter --once --dry-run

test:
	go test ./...

build:
	go build -o bin/jobhunter ./cmd/jobhunter
