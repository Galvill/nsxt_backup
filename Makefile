# Pure Go cross-builds (no CGO). Requires Go toolchain with cross-compilation support.
DIST   ?= dist
BINARY := nsxt-fw-backup
PKG    := ./cmd/nsxt-fw-backup

.PHONY: build build-all clean help

help:
	@echo "Targets:"
	@echo "  build      - build for current OS/arch -> $(DIST)/$(BINARY)"
	@echo "  build-all  - build for linux, windows, darwin (amd64 + arm64)"
	@echo "  clean      - remove $(DIST)/"

build:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 go build -o $(DIST)/$(BINARY) $(PKG)

build-all:
	@mkdir -p $(DIST)
	@set -e; \
	for triplet in \
		"linux amd64 $(BINARY)-linux-amd64" \
		"linux arm64 $(BINARY)-linux-arm64" \
		"windows amd64 $(BINARY)-windows-amd64.exe" \
		"windows arm64 $(BINARY)-windows-arm64.exe" \
		"darwin amd64 $(BINARY)-darwin-amd64" \
		"darwin arm64 $(BINARY)-darwin-arm64"; \
	do \
		set -- $$triplet; \
		os=$$1; arch=$$2; out=$$3; \
		echo "==> $$os/$$arch -> $(DIST)/$$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "$(DIST)/$$out" $(PKG); \
	done
	@echo "Built binaries in $(DIST)/"

clean:
	rm -rf $(DIST)
