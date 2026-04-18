.PHONY: build run test clean

build:
	go build -o bin/gateway ./cmd/server

run: build
	./bin/gateway

test:
	go test -v ./...

clean:
	rm -rf bin/
