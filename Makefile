.PHONY: all build build-ui test clean run

BINARY ?= bin/dokidoki
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

all: build

build: build-ui
	@mkdir -p $(dir $(BINARY))
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/dokidoki

build-ui:
	cd web && bun install && bun run build

test:
	go test -v ./...

clean:
	rm -rf bin/ web/dist

run: build
	./$(BINARY)
