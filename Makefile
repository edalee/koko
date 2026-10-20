.PHONY: dev build sign signing-cert build-cli build-worker test test-worker lint lint-fe typecheck check clean install-fe setup

WAILS := $(HOME)/go/bin/wails
APP := build/bin/Koko.app
# koko-worker reports this with `koko-worker version`: the tag, plus commits
# since it and "-dirty" for a local build.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# The keychain certificate `sign` uses. macOS recognises an ad-hoc signed app by
# its hash, which changes with every build, so it forgets the privacy permissions
# you gave Koko. With a certificate, macOS recognises each build as the same app.
# Create it once with `make signing-cert`.
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

# sign signs koko-worker, then the app around it. Copying koko-worker in breaks
# the signature Wails gives the app, so the app is always signed again. Without
# a trusted certificate, both get a new ad-hoc signature.
sign:
	@if security find-identity -v -p codesigning 2>/dev/null | grep -qF "\"$(SIGN_ID)\""; then \
		id="$(SIGN_ID)"; label="$(SIGN_ID)"; \
	else \
		id=-; label="an ad-hoc signature"; \
		echo "No trusted \"$(SIGN_ID)\" certificate, so Koko.app gets an ad-hoc signature. Run make signing-cert once."; \
	fi; \
	codesign --force --sign "$$id" $(APP)/Contents/MacOS/koko-worker && \
	codesign --force --sign "$$id" $(APP) && \
	echo "Signed Koko.app with $$label"

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
