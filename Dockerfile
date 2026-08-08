# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
RUN apk add --no-cache ca-certificates
WORKDIR /src
ARG VERSION=3.1.0
COPY . .
RUN CGO_ENABLED=0 go test ./... && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/localzet/knotroute/internal/overlay.Version=${VERSION}" -o /out/knotroute ./cmd/knotroute

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/knotroute /knotroute
VOLUME ["/data"]
EXPOSE 7447/tcp 8484/tcp
ENTRYPOINT ["/knotroute"]
CMD ["run", "--config", "/data/knotroute.json"]
