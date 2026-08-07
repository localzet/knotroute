BINARY := knotroute
VERSION := 2.0.0

.PHONY: all build desktop service test race vet fmt clean ui release
all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) ./cmd/knotroute

desktop:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -H=windowsgui" -o bin/knotroute-desktop.exe ./cmd/knotroute-desktop

service:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -H=windowsgui" -o bin/knotroute-service.exe ./cmd/knotroute-service

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
