# KnotRoute 3.1.1 docs/CI correction

This overlay supersedes the hand-written `cmd/knotroute-docs` static site from the initial 3.1.0 UX pack.

## Changes

- KnotRoute Docs is now a real Next.js + MDX + Tailwind documentation application, following the same architectural family as Triangle Docs and localzet/server-docs.
- RU/EN routes, responsive navigation, dark documentation layout, task-first pages, FlexSearch-backed search dialog and MDX components.
- `Dockerfile.docs` builds a standalone Next.js server on Node 24 and exposes `/api/health`.
- Removed the old Go docs binary and embedded `internal/ops/docsite` assets.
- Fixed Control Alpine package name: `libqrencode-tools` provides `qrencode` on Alpine 3.22.
- Fixed Android `label()` Kotlin shadowing/reassignment compile error.
- Upgraded Docker Actions to Node-24-era majors: setup-buildx v4, login v4, metadata v6, build-push v7.
