# Contributing

KnotRoute keeps the core dependency-free and favors auditable protocol code over feature breadth.

Before opening a pull request:

```bash
gofmt -w cmd internal
go vet ./...
go test -race ./...
npm --prefix web run build
git diff --exit-code internal/overlay/assets
```

Protocol changes must update `docs/protocol.md`, add compatibility tests, and explain the threat-model impact. New runtime dependencies require a concrete security or portability justification.

Do not submit generated identities, private keys, production node IDs, or reachable service addresses.
