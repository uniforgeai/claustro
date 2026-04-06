# This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
# Copyright (c) 2026 Uniforge GmbH. All rights reserved.

BINARY := bin/claustro
SHIM_REC := internal/image/rec-shim
SHIM_ARECORD := internal/image/arecord-shim

.PHONY: build build-shims run test lint clean

build: build-shims
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/claustro

build-shims:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(SHIM_REC) ./cmd/rec-shim
	cp $(SHIM_REC) $(SHIM_ARECORD)

run:
	go run ./cmd/claustro $(ARGS)

test:
	CGO_ENABLED=0 go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ $(SHIM_REC) $(SHIM_ARECORD)
