.PHONY: build run test lint clean

build:
	go build -o bin/koko ./cmd/koko

run: build
	./bin/koko

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
