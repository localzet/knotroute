# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go test ./... && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/knotroute ./cmd/knotroute

FROM scratch
COPY --from=build /out/knotroute /knotroute
VOLUME ["/data"]
EXPOSE 7447/tcp 8484/tcp
ENTRYPOINT ["/knotroute"]
CMD ["run", "--config", "/data/knotroute.json"]
