GO ?= go
OPS ?= ops
PORT ?= 8080
ARCH ?= $(shell $(GO) env GOARCH)
HOST_OS := $(shell $(GO) env GOOS)
BINARY := build/vulnapp

.DEFAULT_GOAL := help

.PHONY: help build test check run-native image run-unikernel clean

help:
	@echo "Unikernel conference demo"
	@echo ""
	@echo "  make check           verify Go, QEMU, Ops, and lsof"
	@echo "  make test            run the Go tests"
	@echo "  make run-native      run the vulnerable app on the host"
	@echo "  make image           build a bootable Nanos image"
	@echo "  make run-unikernel   build and boot it with QEMU via Ops"

build:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) $(GO) build -buildvcs=false -trimpath -ldflags="-s -w" -o $(BINARY) .

test:
	CGO_ENABLED=0 $(GO) test -buildvcs=false ./...

check:
	@command -v $(GO) >/dev/null || { echo "missing: go"; exit 1; }
	@command -v $(OPS) >/dev/null || { echo "missing: ops"; exit 1; }
	@command -v lsof >/dev/null || { echo "missing: lsof (Ops uses it to check forwarded ports)"; exit 1; }
	@ops_path=$$(command -v $(OPS)); \
	grep -a -q ops_render_config "$$ops_path" || { \
		echo "This Ops binary does not support config interpolation: $$ops_path"; \
		echo "Ops 0.1.35 or newer is required. Install it with: curl https://ops.city/get.sh -sSfL | sh"; \
		exit 1; \
	}
	@if [ "$(ARCH)" = "arm64" ]; then qemu=qemu-system-aarch64; else qemu=qemu-system-x86_64; fi; \
	command -v $$qemu >/dev/null || { echo "missing: $$qemu"; exit 1; }; \
	echo "Go:   $$($(GO) version)"; \
	echo "QEMU: $$($$qemu --version | head -n 1)"; \
	echo "Ops:  $$(command -v $(OPS))"; \
	echo "lsof: $$(command -v lsof)"; \
	echo "arch: $(ARCH)"

run-native: build
	@if [ "$(HOST_OS)" = "linux" ]; then \
		DEMO_UNSAFE=1 PORT=$(PORT) ./$(BINARY); \
	else \
		DEMO_UNSAFE=1 PORT=$(PORT) $(GO) run -buildvcs=false .; \
	fi

image: build
	DEMO_PORT=$(PORT) ops_render_config=true $(OPS) build -c ops.json $(BINARY)

run-unikernel: build
	DEMO_PORT=$(PORT) ops_render_config=true $(OPS) run -p $(PORT) -c ops.json $(BINARY)

clean:
	rm -f $(BINARY)
