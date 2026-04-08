.PHONY: build run test clean

build:
	CGO_ENABLED=1 go build -o bin/server ./cmd/server

run: build
	bin/server

test:
	go test -race -coverprofile=coverage.out ./...

clean:
	rm -rf bin/ coverage.out
