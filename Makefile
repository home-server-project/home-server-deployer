SHELL := /bin/sh

.PHONY: fmt fmt-check vet test check build test-vm

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f)

fmt-check:
	@test -z "$$(gofmt -l $$(find cmd internal -name '*.go' -type f))" || (gofmt -l $$(find cmd internal -name '*.go' -type f); exit 1)

vet:
	go vet ./...

test:
	go test -race ./...

check: fmt-check vet test

build:
	go build ./cmd/deployer-agent ./cmd/deployer-web

test-vm:
	./tests/vm/run-alpha0.sh preflight
