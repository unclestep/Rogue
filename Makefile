CC=go

CMD=cmd/main.go
APP_NAME=gogue

BUILD_DIR=build
PLATFORMS := \
    linux/amd64 linux/arm64 \
    windows/amd64 windows/386 \
    darwin/amd64 darwin/arm64 \
    freebsd/amd64 freebsd/arm64

VERSION ?= dev

temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))

# -X 'main.Version=$(VERSION)'
LDFLAGS=-ldflags="-s -w"

.PHONY: all build release clean run dump-topologies run-notebook test-rl

all: fmt lint test build

run:
	$(CC) run $(CMD)

test:
	$(CC) test ./...

# Dumps 1000 procedurally-generated dungeons as JSON into rl/topologies/ for the
# Python Pursuer training loop. Re-run whenever the TopologyGenerator changes.
dump-topologies:
	$(CC) run ./cmd/dump-topology -n 1000 -out rl/topologies

# Opens the Pursuer training notebook in local Jupyter.
run-notebook:
	jupyter lab rl/pursuer_training.ipynb

# Runs the Python-side observation/raycaster parity tests. Requires
# `pip install -r rl/requirements.txt` (at least gymnasium + numpy + pytest).
test-rl:
	python -m pytest rl/tests -q

build: $(PLATFORMS)

$(PLATFORMS):
	@mkdir -p $(BUILD_DIR)/$(os)-$(arch)
	@echo "Building for $(os)/$(arch)..."
	GOOS=$(os) GOARCH=$(arch) $(CC) build $(LDFLAGS) -o $(BUILD_DIR)/$(os)-$(arch)/$(APP_NAME)$(if $(findstring windows,$(os)),.exe) $(CMD)

release: clean build
	@mkdir -p $(BUILD_DIR)/dist
	@for dir in $(wildcard $(BUILD_DIR)/*-*) ; do \
		platform=$$(basename $$dir) ; \
		if echo $$platform | grep -q "windows"; then \
			zip -rj $(BUILD_DIR)/dist/$(APP_NAME)-$(VERSION)-$$platform.zip $$dir ; \
		else \
			tar -C $(BUILD_DIR)/$$platform -czf $(BUILD_DIR)/dist/$(APP_NAME)-$(VERSION)-$$platform.tar.gz . ; \
		fi \
	done
	@echo "Release $(VERSION) created in $(BUILD_DIR)/dist"

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

clean:
	rm -rf $(BUILD_DIR)
