SHELL := /bin/bash

GO := go
GOLANGCI_LINT := golangci-lint
GOTESTSUM := $(GO) tool gotest.tools/gotestsum --format=testdox --hide-summary=skipped
GO_LICENSES := $(GO) tool github.com/google/go-licenses/v2

COVERPROFILE := coverage.out

.PHONY: help tools fmt fmt-check lint lint-root check-licenses test clean

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make tools            Install pinned local tooling' \
		'  make fmt              Run golangci-lint fmt on all packages' \
		'  make fmt-check        Check formatting (fails if unformatted)' \
		'  make lint             Run golangci-lint' \
		'  make check-licenses   Check dependency licenses' \
		'  make test             Run unit tests' \
		'  make clean            Remove generated artifacts'

tools:
	mise install

fmt:
	$(GOLANGCI_LINT) fmt ./...

fmt-check:
	@OUT=$$($(GOLANGCI_LINT) fmt ./... --diff 2>&1); \
	if [ -n "$$OUT" ]; then \
		echo "$$OUT"; \
		echo "Run 'make fmt'"; \
		exit 1; \
	fi

lint:
	$(GOLANGCI_LINT) run ./...

check-licenses:
	$(GO_LICENSES) check ./... --include_tests --disallowed_types=unknown,restricted,forbidden

test:
	$(GOTESTSUM) -- -race -shuffle=on -count=1 -covermode=atomic -coverprofile=$(COVERPROFILE) ./...

clean:
	rm -f $(COVERPROFILE)
