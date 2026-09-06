.PHONY: all web build run dev test clean

all: build

web:
	cd web && npm ci --no-audit --no-fund && npm run build

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build: web
	go build -ldflags "$(LDFLAGS)" -o bin/ezlol ./cmd/ezlol

run: build
	./bin/ezlol

# Backend proxies the UI to Vite for hot reload: run `make dev` and `cd web && npm run dev` side by side.
dev:
	EZLOL_DEV=http://localhost:5173 go run ./cmd/ezlol -no-open

test:
	go vet ./... && go test ./...

clean:
	rm -rf bin web/dist/assets web/dist/index.html

# Electron desktop shell (macOS). `make app` runs it against bin/ezlol; `make dmg` packages ezlol.app + .dmg into electron/dist.
app: build
	cd electron && npm install --no-audit --no-fund && npm start

dmg: build
	cd electron && npm install --no-audit --no-fund && npm run dist
