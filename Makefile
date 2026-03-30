# This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
# Copyright (c) 2026 Uniforge GmbH. All rights reserved.

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
