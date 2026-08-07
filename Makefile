BINARY := knotroute
VERSION := 1.0.0

.PHONY: all build test race vet fmt clean ui release
all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) ./cmd/knotroute

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

ui:
	npm --prefix web run build

release:
	VERSION=$(VERSION) ./scripts/build-release.sh

clean:
	rm -rf bin dist
