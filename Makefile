BINARY := knotroute
VERSION ?= 4.0.0-alpha.3
LDFLAGS := -s -w -X github.com/localzet/knotroute/internal/overlay.Version=$(VERSION)

.PHONY: all build beacon sidecar desktop service test race vet fmt clean ui release android
all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/knotroute

beacon:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/knotroute-beacon ./cmd/knotroute-beacon

sidecar:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/knotroute-sidecar ./cmd/knotroute-sidecar

desktop:
	cd cmd/knotroute-desktop/frontend && npm install --no-audit --no-fund --no-package-lock && npm run check && npm run build
	cd cmd/knotroute-desktop && go mod tidy && wails build -clean -platform windows/amd64 -webview2 embed -trimpath -ldflags "$(LDFLAGS)" -o KnotRoute.exe
	mkdir -p bin && cp cmd/knotroute-desktop/build/bin/KnotRoute.exe bin/KnotRoute.exe

service:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS) -H=windowsgui" -o bin/knotroute-service.exe ./cmd/knotroute-service

test:
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
	rm -rf bin dist android/app/build android/.gradle android/app/libs/*.aar cmd/knotroute-desktop/build cmd/knotroute-desktop/frontend/node_modules
