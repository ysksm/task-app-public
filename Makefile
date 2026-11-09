# taskapp build tasks. Requires Go 1.26+, Node 22+ and pnpm.
BIN := bin/taskapp
DATA ?= $(HOME)/.taskapp

.PHONY: all build web go test test-go test-api test-e2e dev-api dev-web demo demo-fresh perf dist clean

all: build

build: web go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

go:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/taskapp

web:
	pnpm --filter taskapp-web build

test: test-go test-api

test-go:
	go test ./...

# Black-box API tests. To test another implementation: make test-api TASKAPP_BIN=/path/to/bin
test-api: go
	cd apitest && TASKAPP_BIN=$(abspath $(or $(TASKAPP_BIN),$(BIN))) pnpm test

# Browser tests of the main screens (Playwright, Chromium). Needs the embedded
# web UI, so it depends on build. First time: cd e2e && pnpm exec playwright install chromium
test-e2e: build
	cd e2e && TASKAPP_BIN=$(abspath $(or $(TASKAPP_BIN),$(BIN))) pnpm test

# Local development: API on :7700 with demo data, Vite dev server on :5173 (proxies /api).
demo: go
	@test -f $(DATA)/taskapp.db || ./$(BIN) seed --data $(DATA) --preset demo

# Fresh demo data in a separate directory (does not touch $(DATA)).
DEMO_DATA ?= /tmp/taskapp-demo
demo-fresh: go
	rm -rf $(DEMO_DATA) && ./$(BIN) seed --data $(DEMO_DATA) --preset demo
	./$(BIN) serve --data $(DEMO_DATA)

dev-api: demo
	./$(BIN) serve --data $(DATA) --log-level debug

dev-web:
	pnpm --filter taskapp-web dev

clean:
	rm -rf dist
	rm -rf bin web/dist/assets web/dist/index.html

# Performance check with ~10,000 issues (NFR-03).
perf: go
	scripts/perf.sh $(BIN)

# Cross-compiled release archives in dist/ (pure-Go SQLite, so CGO is off).
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64
dist: web
	rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; ext=$$( [ $$os = windows ] && echo .exe ); \
	  name=taskapp_$(VERSION)_$${os}_$${arch}; \
	  echo "build $$name"; \
	  mkdir -p dist/$$name && \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$name/taskapp$$ext ./cmd/taskapp && \
	  cp README.md dist/$$name/ && \
	  if [ $$os = windows ]; then (cd dist && zip -qr $$name.zip $$name); else tar -C dist -czf dist/$$name.tar.gz $$name; fi && \
	  rm -rf dist/$$name || exit 1; \
	done
	@cd dist && shasum -a 256 taskapp_* > checksums.txt && ls -la
