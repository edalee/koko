.PHONY: dev build build-cli test lint lint-fe typecheck check clean install-fe setup

WAILS := $(HOME)/go/bin/wails

dev:
	$(WAILS) dev

build:
	$(WAILS) build

build-cli:
	cd cmd/koko-cli && go build -o ../../build/bin/koko-cli .

test:
	go test ./...
	cd frontend && npx vitest run

lint:
	golangci-lint run

lint-fe:
	cd frontend && npx biome check .

typecheck:
	cd frontend && npx tsc --noEmit

check: lint lint-fe typecheck

clean:
	rm -rf build/bin/ frontend/dist/ frontend/node_modules/

install-fe:
	cd frontend && npm install

setup: install-fe
	lefthook install
