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

.PHONY: all build release clean run dump-topologies dump-topologies-full dump-topologies-medium dump-topologies-small run-notebook test-rl

all: fmt lint test build

run:
	$(CC) run $(CMD)

test:
	$(CC) test ./...

dump-topologies: dump-topologies-small dump-topologies-medium dump-topologies-full

dump-topologies-full:
	$(CC) run ./cmd/dump-topology -n 2000 -seed 42 -rooms-h 4 -rooms-v 4 -extra-connections 2 -out rl/fixtures/topologies

dump-topologies-medium:
	$(CC) run ./cmd/dump-topology -n 500 -seed 100 -rooms-h 3 -rooms-v 3 -extra-connections 1 -out rl/fixtures/topologies_medium

dump-topologies-small:
	$(CC) run ./cmd/dump-topology -n 500 -seed 200 -rooms-h 2 -rooms-v 2 -extra-connections 0 -out rl/fixtures/topologies_small

# Opens the Pursuer training notebook in local Jupyter.
run-notebook:
	jupyter lab rl/pursuer_training.ipynb

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
	go vet ./...
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files:"; echo "$$unformatted"; exit 1; \
	fi

fmt:
	gofmt -w .

clean:
	rm -rf $(BUILD_DIR)
