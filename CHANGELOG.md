# Changelog

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
