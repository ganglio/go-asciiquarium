# A tiny virtual aquarium for the Pimoroni Badger 2350.
#
# On a computer (for debugging):  make run   make snapshot   make test
# For the badge (needs TinyGo):   make badger   make flash
# Everything:                     make check

GO     ?= go
TARGET ?= badger2350

# TinyGo builds the firmware. "make install-dependencies" downloads this version
# into $(TOOLS), inside the project, and nothing else on the computer is touched.
# If it is there it is the TinyGo that is used, and if not, the one on the PATH.
TINYGO_VERSION ?= 0.44.0
TOOLS          ?= .tools
TINYGO_DIR     := $(TOOLS)/tinygo-$(TINYGO_VERSION)
LOCAL_TINYGO   := $(TINYGO_DIR)/tinygo/bin/tinygo
TINYGO         ?= $(if $(wildcard $(LOCAL_TINYGO)),$(LOCAL_TINYGO),tinygo)

# Which TinyGo build is the one for this computer.
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_S)-$(UNAME_M),Darwin-arm64)
TINYGO_PLATFORM := darwin-arm64
else ifeq ($(UNAME_S)-$(UNAME_M),Darwin-x86_64)
TINYGO_PLATFORM := darwin-amd64
else ifeq ($(UNAME_S)-$(UNAME_M),Linux-x86_64)
TINYGO_PLATFORM := linux-amd64
else ifneq (,$(filter $(UNAME_S)-$(UNAME_M),Linux-aarch64 Linux-arm64))
TINYGO_PLATFORM := linux-arm64
endif
TINYGO_URL := https://github.com/tinygo-org/tinygo/releases/download/v$(TINYGO_VERSION)/tinygo$(TINYGO_VERSION).$(TINYGO_PLATFORM).tar.gz

# The SHA-256 of each download of the version above, as published with the
# release. For another TINYGO_VERSION give TINYGO_SHA256, or it is not checked.
TINYGO_PINNED                := 0.44.0
TINYGO_SHA256_darwin-arm64   := 696a37db149c2c37ed0da96cc1f2cca2630fca0c4dd19823a2a481c9b4a6a748
TINYGO_SHA256_darwin-amd64   := 405b8241e20825cd3e7267706eb27c4f40fe372f3548a4d7f80ef2eb640957a7
TINYGO_SHA256_linux-amd64    := ed0f45543a49ff2a99120a5a1b8814a0750e1fda1a66f8e2a0a172576134aaf9
TINYGO_SHA256_linux-arm64    := 48cece9e1e886a176caad0918182d90f8348ad3084774e6239f42e5e11e40990
ifeq ($(TINYGO_VERSION),$(TINYGO_PINNED))
TINYGO_SHA256 ?= $(TINYGO_SHA256_$(TINYGO_PLATFORM))
endif
SHA256SUM := $(if $(shell command -v sha256sum),sha256sum,shasum -a 256)

BIN := bin
UF2 := $(BIN)/aquarium.uf2

# Settings for the badge, which replace the defaults when they are given:
#   make flash IDLE=2m NIGHT=23:00-06:30 NIGHT_INTERVAL=5m NIGHT_PEEK=1m
IDLE           ?=
TAIL           ?=
NIGHT          ?=
NIGHT_INTERVAL ?=
NIGHT_PEEK     ?=

# What is built into the firmware: the settings above, and the time of the build,
# which is how the badge's clock gets set (see cmd/badger/clock.go).
LDFLAGS = -ldflags "-X main.buildTime=$$(date +%Y-%m-%dT%H:%M:%S) -X main.paceIdle=$(IDLE) -X main.paceTail=$(TAIL) -X main.paceNight=$(NIGHT) -X main.paceNightInterval=$(NIGHT_INTERVAL) -X main.paceNightPeek=$(NIGHT_PEEK)"

# Extra arguments for "make run", "make run-badge" and "make snapshot", e.g.
#   make run ARGS="-seed 42 -time 21:59:50"
#   make run-badge ARGS="-refresh 0"
#   make snapshot ARGS="-feed -steps 1"
ARGS ?=

.DEFAULT_GOAL := help

.PHONY: help
help: ## show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  make %-21s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Setting up

.PHONY: install install-dependencies
install: install-dependencies

install-dependencies: ## install what the badge build needs: TinyGo, into .tools/ (no sudo)
	@command -v $(GO) >/dev/null || { echo "Go is needed first, from https://go.dev/dl/"; exit 1; }
	@test -n "$(TINYGO_PLATFORM)" || { echo "There is no TinyGo download for $(UNAME_S) $(UNAME_M): see https://tinygo.org/getting-started/install/"; exit 1; }
	@if [ -x "$(LOCAL_TINYGO)" ]; then \
	  echo "TinyGo $(TINYGO_VERSION) is already in $(TINYGO_DIR)"; \
	else \
	  set -e; mkdir -p "$(TINYGO_DIR)"; archive="$(TINYGO_DIR)/download.tar.gz"; \
	  echo "Downloading $(TINYGO_URL)"; \
	  curl -fL --progress-bar -o "$$archive" "$(TINYGO_URL)"; \
	  if [ -n "$(TINYGO_SHA256)" ]; then \
	    echo "$(TINYGO_SHA256)  $$archive" | $(SHA256SUM) -c - >/dev/null \
	      || { rm -f "$$archive"; echo "The download does not match its checksum: not installing it."; exit 1; }; \
	    echo "Checksum ok"; \
	  else \
	    echo "No checksum for this version: the download is not verified."; \
	  fi; \
	  tar -xzf "$$archive" -C "$(TINYGO_DIR)"; rm -f "$$archive"; \
	fi
	@"$(LOCAL_TINYGO)" version
	@echo "Done: make now uses it by itself. Try: make check"

##@ On this computer

.PHONY: run
run: ## run in the terminal, in real time: f feeds the fish, r makes a new tank, q quits
	$(GO) run ./cmd/aquarium $(ARGS)

.PHONY: run-badge
run-badge: ## run in the terminal at the badge's pace, with its e-ink screen simulated
	$(GO) run ./cmd/aquarium -badge $(ARGS)

.PHONY: snapshot
snapshot: ## draw the badge's screen to bin/snapshot.png (try ARGS="-feed -steps 1")
	@mkdir -p $(BIN)
	$(GO) run ./cmd/aquarium -snapshot $(BIN)/snapshot.png $(ARGS)

.PHONY: test
test: ## run the tests
	$(GO) test ./...

.PHONY: test-short
test-short: ## run the tests, with shorter simulations
	$(GO) test -short ./...

.PHONY: vet
vet: ## run go vet and check the formatting
	$(GO) vet ./...
	@test -z "$$(gofmt -l cmd internal)" || { echo "needs gofmt:"; gofmt -l cmd internal; exit 1; }

##@ For the badge

.PHONY: badger
badger: $(UF2) ## build the firmware: bin/aquarium.uf2

# The sources are not listed: let TinyGo decide whether anything changed.
.PHONY: $(UF2)
$(UF2):
	@command -v $(TINYGO) >/dev/null || { echo "TinyGo not found: run make install-dependencies, or install it yourself (https://tinygo.org/getting-started/install/) or run make TINYGO=/path/to/tinygo"; exit 1; }
	@mkdir -p $(BIN)
	$(TINYGO) build -target=$(TARGET) $(LDFLAGS) -o $@ ./cmd/badger

.PHONY: flash
flash: ## build and copy the firmware to the badge (put it in boot mode first, see README)
	$(TINYGO) flash -target=$(TARGET) $(LDFLAGS) ./cmd/badger

.PHONY: size
size: ## how much flash and RAM the firmware takes
	@mkdir -p $(BIN)
	$(TINYGO) build -target=$(TARGET) $(LDFLAGS) -size short -o $(BIN)/size.uf2 ./cmd/badger

.PHONY: test-tinygo
test-tinygo: ## run the tests with TinyGo's compiler and runtime, which is what the badge runs
	$(TINYGO) test ./internal/...

##@ Everything

.PHONY: check
check: vet test badger test-tinygo ## vet, test, and build the firmware

.PHONY: clean
clean: ## remove the build output
	rm -rf $(BIN)
