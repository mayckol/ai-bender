.PHONY: build test lint install smoke clean fmt tidy ui-assets ui-test

GO            ?= go
GOLANGCI_LINT ?= golangci-lint
SHELLCHECK    ?= shellcheck
BIN           := bin/bender
PKG           := ./...
INSTALL_DIR   ?= /usr/local/bin
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -X github.com/mayckol/ai-bender/internal/version.Version=$(VERSION)

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/bender

test:
	$(GO) test -race -count=1 $(PKG)

lint:
	$(GOLANGCI_LINT) run $(PKG)
	@$(SHELLCHECK) .specify/scripts/bash/*.sh 2>/dev/null || true

fmt:
	$(GO) fmt $(PKG)
	$(GO) vet $(PKG)

tidy:
	$(GO) mod tidy

install: build
	install -m 0755 $(BIN) $(INSTALL_DIR)/bender

smoke: build
	$(GO) test -race -count=1 -tags=smoke ./tests/integration/...

ui-assets:
	cd ui && bun install --silent && bun run build:embed

ui-test:
	cd ui && bun install --silent && bun test

clean:
	rm -rf bin/ dist/ coverage.txt coverage.html ui/node_modules/
