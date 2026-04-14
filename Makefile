PREFIX ?= /usr/local
BINARY = sshtui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/diegovrocha/sshtui/internal/ui.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/sshtui

install: build
	@mkdir -p $(PREFIX)/bin
	@cp $(BINARY) $(PREFIX)/bin/$(BINARY)
	@chmod +x $(PREFIX)/bin/$(BINARY)
	@echo "✔ sshtui installed at $(PREFIX)/bin/$(BINARY)"

uninstall:
	@rm -f $(PREFIX)/bin/$(BINARY)
	@echo "✔ sshtui removed"

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./... -count=1

vet:
	go vet ./...

check: vet test

# ─── Release targets ────────────────────────────────────
# Runs scripts/bump.sh to tag a new version and push to GitHub.
# GitHub Actions + GoReleaser then publish binaries automatically.
release-patch:
	@./scripts/bump.sh patch

release-minor:
	@./scripts/bump.sh minor

release-major:
	@./scripts/bump.sh major

# Auto-detects patch/minor/major from conventional commit messages
# since the last tag. Use this after a coding session.
release-auto:
	@./scripts/bump.sh auto

# Usage: make release VERSION=1.5.0
release:
	@[ -n "$(VERSION)" ] || { echo "Usage: make release VERSION=X.Y.Z"; exit 1; }
	@./scripts/bump.sh $(VERSION)

.PHONY: build install uninstall run clean test vet check \
	release release-patch release-minor release-major release-auto
