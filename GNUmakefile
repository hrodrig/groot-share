# gfs — build, test, release scaffolding (GNU Make).
# Feature code lands via GSD against docs/SPECIFICATIONS.md.

APP_NAME := gfs
BIN_DIR  := bin
MODULE   := github.com/hrodrig/groot-share

VERSION_RAW ?= $(shell cat VERSION 2>/dev/null | tr -d '\n\r')
VERSION     := $(patsubst v%,%,$(VERSION_RAW))
TAG         := v$(VERSION)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BRANCH      := $(shell git symbolic-ref --short HEAD 2>/dev/null || echo unknown)
BUILDDATE   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT)' \
	-X 'main.branch=$(BRANCH)' \
	-X 'main.buildDate=$(BUILDDATE)'

GOLANGCI_LINT_VERSION ?= v2.12.2
# Gate like groot: release-check runs cover; override locally with COVER_MIN=0 if needed.
COVER_MIN ?= 80
GRYPE_FAIL_ON ?= high
GRYPE_DIR_EXCLUDES := --exclude './bin/**' --exclude './dist/**' --exclude './.gocache/**' --exclude './.gotmp/**'
STRICT_RELEASE ?= 0
IMAGE     ?= $(APP_NAME):local
IMAGE_AMD64 ?= $(IMAGE)-amd64
PLATFORMS ?= linux/amd64,linux/arm64
DIST          := dist
FREEBSD_ARCH  ?= amd64
OPENBSD_ARCH  ?= amd64
ENV_FILE      ?= .env

check-docker = @docker info >/dev/null 2>&1 || { echo "Error: Docker is not running. Start Docker and try again."; exit 1; }

DOCKER_BUILD_ARGS := \
	--build-arg APP_VERSION=$(VERSION) \
	--build-arg GIT_COMMIT=$(COMMIT) \
	--build-arg GIT_BRANCH=$(BRANCH) \
	--build-arg BUILD_DATE=$(BUILDDATE)

GREEN  := \033[0;32m
YELLOW := \033[0;33m
CYAN   := \033[0;36m
RESET  := \033[0m
ifneq ($(NO_COLOR),)
  GREEN  :=
  YELLOW :=
  CYAN   :=
  RESET  :=
endif

.DEFAULT_GOAL := help

.PHONY: help all build test cover fmt fmt-check lint-fix lint vet run serve clean install \
	docker-build docker-build-amd64 docker-scan goreleaser-check release-check ci \
	gocyclo govulncheck vulncheck grype security \
	dist-freebsd dist-openbsd port-freebsd-sync port-openbsd-sync man-sync

help:
	@echo "$(GREEN)gfs$(RESET) — groot files share"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "$(YELLOW)Build:$(RESET)"
	@echo "  $(GREEN)build$(RESET)              Build $(BIN_DIR)/$(APP_NAME)"
	@echo "  $(GREEN)run$(RESET)                Run $(BIN_DIR)/$(APP_NAME) (env from shell)"
	@echo "  $(GREEN)serve$(RESET)              Run with $(ENV_FILE) (cp .env.example $(ENV_FILE))"
	@echo "  $(GREEN)install$(RESET)            Install to GOPATH/bin"
	@echo "  $(GREEN)clean$(RESET)              Remove $(BIN_DIR)/$(APP_NAME) and dist/"
	@echo ""
	@echo "$(YELLOW)Test / quality:$(RESET)"
	@echo "  $(GREEN)test$(RESET)               go test ./... -race -count=1"
	@echo "  $(GREEN)cover$(RESET)              Coverage; gate COVER_MIN=$(COVER_MIN)"
	@echo "  $(GREEN)fmt-check$(RESET)          Fail if gofmt -s would change files"
	@echo "  $(GREEN)lint$(RESET)               golangci-lint ($(GOLANGCI_LINT_VERSION))"
	@echo "  $(GREEN)vet$(RESET)                go vet ./..."
	@echo "  $(GREEN)gocyclo$(RESET)            Fail if complexity > 14"
	@echo "  $(GREEN)ci$(RESET)                 fmt-check + lint + gocyclo + test"
	@echo ""
	@echo "$(YELLOW)Security:$(RESET)"
	@echo "  $(GREEN)govulncheck$(RESET)        golang.org/x/vuln"
	@echo "  $(GREEN)grype$(RESET)              Directory scan (fail-on $(GRYPE_FAIL_ON))"
	@echo "  $(GREEN)security$(RESET)           govulncheck + gocyclo + grype"
	@echo ""
	@echo "$(YELLOW)Docker / release:$(RESET)"
	@echo "  $(GREEN)docker-build$(RESET)       Local image ($(IMAGE))"
	@echo "  $(GREEN)docker-scan$(RESET)        docker-build + Grype image scan"
	@echo "  $(GREEN)goreleaser-check$(RESET)   goreleaser check"
	@echo "  $(GREEN)release-check$(RESET)      VERSION + goreleaser + fmt-check + lint + cover + security"
	@echo ""
	@echo "$(YELLOW)BSD ports:$(RESET)"
	@echo "  $(GREEN)man-sync$(RESET)            Bump .TH in contrib/man from VERSION"
	@echo "  $(GREEN)port-freebsd-sync$(RESET)   PORTVERSION in contrib/freebsd/Makefile"
	@echo "  $(GREEN)port-openbsd-sync$(RESET)   Version fields in contrib/openbsd/port/Makefile"
	@echo "  $(GREEN)dist-freebsd$(RESET)        Tarball FREEBSD_ARCH=$(FREEBSD_ARCH)"
	@echo "  $(GREEN)dist-openbsd$(RESET)        Tarball OPENBSD_ARCH=$(OPENBSD_ARCH)"
	@echo ""
	@echo "$(CYAN)Contract:$(RESET) docs/SPECIFICATIONS.md  |  VERSION=$(VERSION)  |  COVER_MIN=$(COVER_MIN)"

all: build

build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/$(APP_NAME)

install: build
	install -m 755 $(BIN_DIR)/$(APP_NAME) $$(go env GOPATH)/bin/$(APP_NAME)

clean:
	rm -rf $(BIN_DIR)/$(APP_NAME) dist coverage.out

run: build
	$(BIN_DIR)/$(APP_NAME)

serve: build
	@test -f $(ENV_FILE) || { echo "Error: $(ENV_FILE) missing — cp .env.example $(ENV_FILE)"; exit 1; }
	@set -a && . ./$(ENV_FILE) && set +a && mkdir -p "$$GFS_DATA_DIR" && exec $(BIN_DIR)/$(APP_NAME)

test:
	go test ./... -race -count=1

cover:
	go test ./... -race -count=1 -covermode=atomic -coverpkg=./... -coverprofile=coverage.out
	@P=$$(go tool cover -func=coverage.out | tail -1 | sed 's/^.*[[:space:]]\([0-9.]*\)%.*/\1/'); \
		echo "total (merged) statement coverage: $$P% (minimum $(COVER_MIN)%)"; \
		if [ "$(COVER_MIN)" -gt 0 ]; then \
			command -v bc >/dev/null 2>&1 || { echo "COVER_MIN>0 requires bc"; exit 1; }; \
			if [ "$$(echo "$$P < $(COVER_MIN)" | bc)" -eq 1 ]; then \
				echo "coverage below $(COVER_MIN)%"; exit 1; \
			fi; \
		fi

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -s -l .); if [ -n "$$out" ]; then echo "Run: make lint-fix"; echo "$$out"; exit 1; fi

lint-fix:
	gofmt -s -w .

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...; \
	fi

vet:
	go vet ./...

gocyclo:
	go run github.com/fzipp/gocyclo/cmd/gocyclo@latest -over 14 .

govulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

vulncheck: govulncheck

grype:
	@if command -v grype >/dev/null 2>&1; then \
		grype dir:. $(GRYPE_DIR_EXCLUDES) --fail-on $(GRYPE_FAIL_ON); \
	else \
		echo "grype not found locally, using container image..."; \
		docker run --rm --pull=always -v "$(CURDIR):/workspace" anchore/grype:latest \
			dir:/workspace $(GRYPE_DIR_EXCLUDES) --fail-on $(GRYPE_FAIL_ON); \
	fi

security: govulncheck gocyclo grype
	@echo "$(GREEN)OK: security (govulncheck, gocyclo, grype)$(RESET)"

ci: fmt-check lint gocyclo test
	@echo "$(GREEN)OK: ci (fmt-check, lint, gocyclo, test)$(RESET)"

docker-build:
	$(check-docker)
	docker build $(DOCKER_BUILD_ARGS) -t $(IMAGE) .

docker-build-amd64:
	$(check-docker)
	docker buildx build $(DOCKER_BUILD_ARGS) --platform linux/amd64 --load -t $(IMAGE_AMD64) .

docker-scan: docker-build
	@command -v grype >/dev/null 2>&1 || { echo "grype required: https://github.com/anchore/grype"; exit 1; }
	grype docker:$(IMAGE) --fail-on $(GRYPE_FAIL_ON)

goreleaser-check:
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser required: https://goreleaser.com/install/"; exit 1; }
	@if git remote get-url origin >/dev/null 2>&1; then \
		goreleaser check; \
	else \
		echo "$(YELLOW)goreleaser check skipped: no git remote origin (local-first)$(RESET)"; \
	fi

# goreleaser-check skips gracefully only if origin is missing.
release-check:
	@test -f VERSION || { echo "VERSION file is required"; exit 1; }
	@echo "Release version: $(VERSION) (tag: $(TAG))"
	@echo "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "VERSION must be semver (e.g. 0.1.0)"; exit 1; }
	@$(MAKE) goreleaser-check
	@$(MAKE) fmt-check
	@$(MAKE) lint
	@$(MAKE) cover
	@$(MAKE) security
	@if [ "$(STRICT_RELEASE)" = "1" ]; then $(MAKE) docker-scan; fi
	@echo "$(GREEN)release-check OK$(RESET)"

man-sync:
	@today=$$(date +%Y-%m-%d); \
	f=contrib/man/man1/gfs.1; \
	test -f "$$f" || { echo "Error: $$f not found"; exit 1; }; \
	sed -i.bak "s/^\.TH .*/.TH GFS 1 \"$$today\" \"gfs v$(VERSION)\" \"User Commands\"/" "$$f"; \
	rm -f "$$f.bak"; \
	echo "Updated $$f .TH to gfs v$(VERSION) ($$today)"

port-freebsd-sync:
	@[ -n "$(VERSION)" ] || { echo "Error: VERSION file empty or missing"; exit 1; }
	@sed -i.bak "s/^PORTVERSION=.*/PORTVERSION=\t$(VERSION)/" contrib/freebsd/Makefile
	@rm -f contrib/freebsd/Makefile.bak
	@echo "Updated contrib/freebsd/Makefile PORTVERSION to $(VERSION)"

port-openbsd-sync:
	@[ -n "$(VERSION)" ] || { echo "Error: VERSION file empty or missing"; exit 1; }
	@test -f contrib/openbsd/port/Makefile || { echo "Error: contrib/openbsd/port/Makefile not found"; exit 1; }
	@sed -i.bak \
	  -e 's#^DISTNAME =.*#DISTNAME =\tgfs_v$(VERSION)_openbsd_$${MACHINE_ARCH:S/aarch64/arm64/}#' \
	  -e 's#^PKGNAME =.*#PKGNAME =\tgfs-$(VERSION)#' \
	  -e 's#^MASTER_SITES =.*#MASTER_SITES =\thttps://github.com/hrodrig/groot-share/releases/download/v$(VERSION)/#' \
	  -e 's#^DISTFILES =.*#DISTFILES =\tgfs_v$(VERSION)_openbsd_$${MACHINE_ARCH:S/aarch64/arm64/}.tar.gz#' \
	  contrib/openbsd/port/Makefile
	@rm -f contrib/openbsd/port/Makefile.bak
	@echo "Updated contrib/openbsd/port/Makefile to $(VERSION)"

dist-freebsd:
	@set -e; \
	ver="$(VERSION)"; \
	[ -n "$$ver" ] || { echo "Error: VERSION file is required"; exit 1; }; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver)"; exit 1; }; \
	echo "$(FREEBSD_ARCH)" | grep -qE '^(amd64|arm64)$$' || { echo "Error: FREEBSD_ARCH must be amd64 or arm64"; exit 1; }; \
	arch="$(FREEBSD_ARCH)"; \
	out="$(DIST)/gfs_v$${ver}_freebsd_$$arch.tar.gz"; \
	stage="/tmp/gfs-dist-root-$$PPID"; \
	tmpbin="$(DIST)/gfs-freebsd-$$arch-$$PPID"; \
	echo "Building gfs for FreeBSD $$arch with VERSION=v$$ver..."; \
	mkdir -p "$(DIST)"; \
	GOOS=freebsd GOARCH="$$arch" go build -trimpath -ldflags "$(LDFLAGS)" -o "$$tmpbin" ./cmd/gfs; \
	rm -rf "$$stage"; \
	mkdir -p "$$stage/share/doc/gfs" "$$stage/share/man/man1"; \
	cp "$$tmpbin" "$$stage/gfs"; \
	rm -f "$$tmpbin"; \
	cp LICENSE "$$stage/share/doc/gfs/LICENSE"; \
	cp README.md "$$stage/share/doc/gfs/README.md"; \
	cp contrib/man/man1/gfs.1 "$$stage/share/man/man1/gfs.1"; \
	tar -C "$$stage" -czf "$$out" .; \
	rm -rf "$$stage"; \
	echo "Wrote $$out"

dist-openbsd:
	@set -e; \
	ver="$(VERSION)"; \
	[ -n "$$ver" ] || { echo "Error: VERSION file is required"; exit 1; }; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver)"; exit 1; }; \
	echo "$(OPENBSD_ARCH)" | grep -qE '^(amd64|arm64)$$' || { echo "Error: OPENBSD_ARCH must be amd64 or arm64"; exit 1; }; \
	arch="$(OPENBSD_ARCH)"; \
	out="$(DIST)/gfs_v$${ver}_openbsd_$$arch.tar.gz"; \
	stage="/tmp/gfs-openbsd-dist-root-$$PPID"; \
	tmpbin="$(DIST)/gfs-openbsd-$$arch-$$PPID"; \
	echo "Building gfs for OpenBSD $$arch with VERSION=v$$ver..."; \
	mkdir -p "$(DIST)"; \
	GOOS=openbsd GOARCH="$$arch" go build -trimpath -ldflags "$(LDFLAGS)" -o "$$tmpbin" ./cmd/gfs; \
	rm -rf "$$stage"; \
	mkdir -p "$$stage/share/doc/gfs" "$$stage/share/man/man1"; \
	cp "$$tmpbin" "$$stage/gfs"; \
	rm -f "$$tmpbin"; \
	cp LICENSE "$$stage/share/doc/gfs/LICENSE"; \
	cp README.md "$$stage/share/doc/gfs/README.md"; \
	cp contrib/man/man1/gfs.1 "$$stage/share/man/man1/gfs.1"; \
	tar -C "$$stage" -czf "$$out" .; \
	rm -rf "$$stage"; \
	echo "Wrote $$out"
