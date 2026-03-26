BINARY := bin/claustro

.PHONY: build run test lint clean

build:
	go build -o $(BINARY) ./cmd/claustro

run:
	go run ./cmd/claustro $(ARGS)

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
