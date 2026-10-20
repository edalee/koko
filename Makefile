.PHONY: dev build sign signing-cert build-cli build-worker test test-worker lint lint-fe typecheck check clean install-fe setup

WAILS := $(HOME)/go/bin/wails
APP := build/bin/Koko.app
# koko-worker reports this with `koko-worker version`: the tag, plus commits
# since it and "-dirty" for a local build.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# The keychain certificate `sign` uses. An ad-hoc signature changes with every
# build, so macOS forgets the privacy permissions you gave Koko. A certificate
# keeps one signature. Create it once with `make signing-cert`.
SIGN_ID ?= Koko Local

# Koko finds koko-worker in build/bin during `make dev`, and next to its own
# binary inside Koko.app after `make build`.
dev: build-worker
	$(WAILS) dev

build:
	$(WAILS) build
	$(MAKE) build-worker
	cp build/bin/koko-worker $(APP)/Contents/MacOS/koko-worker
	$(MAKE) sign

# sign signs koko-worker, then the app around it. Without the certificate, or
# off macOS, the app keeps the ad-hoc signature Wails gives it.
sign:
	@if command -v security >/dev/null && security find-certificate -c "$(SIGN_ID)" >/dev/null 2>&1; then \
		codesign --force --sign "$(SIGN_ID)" $(APP)/Contents/MacOS/koko-worker && \
		codesign --force --sign "$(SIGN_ID)" $(APP) && \
		echo "Signed Koko.app with $(SIGN_ID)"; \
	else \
		echo "No \"$(SIGN_ID)\" certificate, so Koko.app keeps its ad-hoc signature. Run make signing-cert once."; \
	fi

signing-cert:
	./scripts/make-signing-cert.sh "$(SIGN_ID)"

build-cli:
	cd cmd/koko-cli && go build -o ../../build/bin/koko-cli .

build-worker:
	cd cmd/koko-worker && go build -ldflags "-X main.version=$(VERSION)" -o ../../build/bin/koko-worker .

# koko-worker is its own Go module, so the root `go test ./...` skips it.
test: test-worker
	go test ./...
	cd frontend && npx vitest run

test-worker:
	cd cmd/koko-worker && go test ./...

lint:
	golangci-lint run
	cd cmd/koko-worker && golangci-lint run ./...

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
