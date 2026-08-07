# Changelog

## 3.0.1

- Added role-based operating guides for end users, service operators, server operators, network operators, and developers.
- Documented the distinction between network isolation and membership authorization, and between direct-service ACLs and anonymous published-service access control.
- Include the complete `docs/` directory in platform release archives.
- Fixed Android CI/release builds to use the stable Android 36 SDK consistently with `targetSdk = 36`.

## 3.0.0

- Added independent Ed25519 service identities and versioned service `.knot` addresses.
- Added signed service descriptors replicated to XOR-nearest directory nodes.
- Added introduction points and rendezvous connections for published hidden services.
- Added telescopically constructed onion circuits with per-hop X25519/AEAD keys.
- Added a second end-to-end X25519/AES-256-GCM session across hidden-service rendezvous paths.
- Added `network_id` isolation and signed network invitation bundles.
- Added multi-source automatic peer discovery: Beacon, LAN multicast, PEX, and persistent cache.
- Added `knotroute-beacon`, including an optional bundled bootstrap relay and Docker image.
- Added a local per-device KnotRoute Root CA and short-lived certificates restricted to `.knot` names.
- Added Windows tray integration that installs/removes the per-user CA and preserves/restores PAC configuration.
- Added Android app sources using the same Go core through gomobile, WebView process proxying, foreground service operation, and explicit CA installation.
- Added `pkg/knotclient`, `pkg/knotserver`, and the Android AAR binding for embedded deployments.
- Added `knotroute-sidecar` and Docker examples for publishing existing container services without host application ports.
- Added reusable RPC, datagram, encrypted pub/sub, content-addressed object, and encrypted offline-mailbox primitives.
- Updated dashboard/status surfaces for networks, circuits, descriptors, introduction points, discovery, CA, and v3 service identities.

## 2.0.0

- Added versioned, checksummed, self-authenticating `.knot` node addresses.
- Added `service.<node>.knot` routing and context-sensitive default services.
- Added local human-readable aliases and signed alias export/import records.
- Added local SOCKS5 CONNECT gateway with remote `.knot` name resolution.
- Added HTTP proxy and CONNECT gateway with optional direct fallback.
- Added a PAC endpoint that selects only `.knot` destinations.
- Replaced the read-only dashboard with a full local configuration interface.
- Added validated atomic configuration updates and in-process reload/shutdown controls.
- Added a dependency-free native Windows tray controller.
- Added per-user Windows installation, startup, PAC integration, setting backup, and restoration.
- Added a native Windows Service Control Manager wrapper and corrected service packaging.
- Added HTTP and SOCKS integration tests across a real A → B → C relay path.
- Added bounded non-blocking relay retries for stream-control packets during link-state convergence.

## 1.0.0

- Initial encrypted multi-hop service overlay.
