# Changelog

## 4.0.0-alpha.1

- Replaced the Windows daemon-plus-dashboard client model with a native one-process `KnotRoute.exe`.
- Added `ku_` user identities, signed profiles/messages/posts, online messenger transport and feed synchronization over `kr-chat`.
- Added a service catalog to desktop and Android clients.
- Added a pluggable transport manager with direct and SOCKS5/Xray peer dialing.
- Rebuilt the Android application around Material UI and removed the embedded WebView browser.
- Added Android browser integration beta through a browser-scoped VPN HTTP proxy on Android 10+.
- Replaced Android CA installation intent with certificate export plus system Settings flow.
- Added configurable X.509 Root CA subject/validity, inspection and explicit rotation.
- Added developer-focused desktop documentation for identities, services, aliases and Docker sidecars.

## Unreleased — stabilization

- Restored HTTP/SOCKS `.knot` proxy routing through the circuit API and added a local-service fast path for TCP and embedded `pipe://` services.
- Added end-to-end Beacon auto-peering integration coverage and stricter Beacon HTTP API URL validation, including a clear rejection of relay port `7447`.
- Made Control accept networks without a Beacon and treat already-running external Beacons as first-class infrastructure; added external Beacon health checks and a dedicated registration flow.
- Made Control state updates transactional, improved management error responses, removed stale frontend caching, and compacted the administration UI with advanced options collapsed by default.
- Hardened Control↔Agent enrollment, signed heartbeat/job handling, generated Compose validation, release-channel image selection, and managed sidecar redeploy behavior.
- Made managed sidecars regenerate runtime configuration from environment on redeploy while preserving stable identities under `/data`.
- Added Russian Windows tray/installer/dashboard text, daemon preflight validation, bounded restart/shutdown sequencing, crash reporting, and bounded automatic recovery.
- Fixed Android API 36 system-bar insets, core restart sequencing, post-start WebView proxy application, Beacon validation, Russian strings, and advanced settings disclosure.
- Expanded RU/EN operational diagnostics and first-network documentation.
- Updated CI action majors and added integration stress/docs production-build jobs.
- Added system CA roots to the generic scratch container so HTTPS Beacon discovery works from the container image.

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
