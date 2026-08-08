BINARY := knotroute
VERSION ?= $(shell cat VERSION)
LDFLAGS := -s -w -X github.com/localzet/knotroute/internal/overlay.Version=$(VERSION)

.PHONY: all build beacon sidecar desktop service test race vet fmt clean ui release android check-version
all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/knotroute

beacon:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/knotroute-beacon ./cmd/knotroute-beacon

sidecar:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/knotroute-sidecar ./cmd/knotroute-sidecar

desktop:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS) -H=windowsgui" -o bin/knotroute-desktop.exe ./cmd/knotroute-desktop

service:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS) -H=windowsgui" -o bin/knotroute-service.exe ./cmd/knotroute-service

check-version:
	sh ./scripts/check-version.sh

test: check-version
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal pkg mobile

ui:
	npm --prefix web run build

release:
	VERSION=$(VERSION) ./scripts/build-release.sh

android:
	VERSION=$(VERSION) ./scripts/build-android.sh

clean:
	rm -rf bin dist android/app/build android/.gradle android/app/libs/*.aar
