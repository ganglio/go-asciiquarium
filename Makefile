# A tiny virtual aquarium for the Pimoroni Badger 2350.
#
# On a computer (for debugging):  make run   make snapshot   make test
# For the badge (needs TinyGo):   make badger   make flash
# Everything:                     make check

GO     ?= go
TINYGO ?= tinygo
TARGET ?= badger2350

BIN := bin
UF2 := $(BIN)/aquarium.uf2

# Settings for the badge, which replace the defaults when they are given:
#   make flash IDLE=2m NIGHT=23:00-06:30 NIGHT_INTERVAL=5m
IDLE           ?=
TAIL           ?=
NIGHT          ?=
NIGHT_INTERVAL ?=

# What is built into the firmware: the settings above, and the time of the build,
# which is how the badge's clock gets set (see cmd/badger/clock.go).
LDFLAGS = -ldflags "-X main.buildTime=$$(date +%Y-%m-%dT%H:%M:%S) -X main.paceIdle=$(IDLE) -X main.paceTail=$(TAIL) -X main.paceNight=$(NIGHT) -X main.paceNightInterval=$(NIGHT_INTERVAL)"

# Extra arguments for "make run" and "make snapshot", e.g.
#   make run ARGS="-seed 42 -refresh 0"
#   make snapshot ARGS="-feed -steps 1"
ARGS ?=

.DEFAULT_GOAL := help

.PHONY: help
help: ## show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  make %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ On this computer

.PHONY: run
run: ## run in the terminal: f feeds the fish, r makes a new tank, q quits
	$(GO) run ./cmd/aquarium $(ARGS)

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
	@command -v $(TINYGO) >/dev/null || { echo "TinyGo not found: install it from https://tinygo.org/getting-started/install/ or run make TINYGO=/path/to/tinygo"; exit 1; }
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
