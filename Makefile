.PHONY: dev build test lint clean install-fe

WAILS := $(HOME)/go/bin/wails

dev:
	$(WAILS) dev

build:
	$(WAILS) build

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf build/bin/ frontend/dist/ frontend/node_modules/

install-fe:
	cd frontend && npm install
