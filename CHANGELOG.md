# Changelog

## 1.0.0 — 2026-08-05

Initial complete release.

- self-certifying Ed25519 node identities;
- mutually authenticated TLS 1.3 peer links;
- signed expiring link-state advertisements;
- bidirectional-edge validation and multi-hop shortest-path routing;
- end-to-end X25519/HKDF-SHA-256/AES-256-GCM TCP streams;
- named service publication, source-ID ACLs, and local forwards;
- reconnecting seed peers, duplicate-link resolution, TTL, and packet deduplication;
- embedded read-only dashboard and status API;
- CLI initialization, validation, runtime, and identity commands;
- race-tested three-node A → B → C integration test;
- static release scripts, Docker, systemd, Windows service, and GitHub Actions.
