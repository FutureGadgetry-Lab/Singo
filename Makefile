.PHONY: build test

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/singo .

test:
	go test ./...
