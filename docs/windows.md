# KnotRoute on Windows

## Desktop installation

Extract the Windows release archive and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-KnotRoute.ps1
```

The per-user installer copies the desktop binaries under:

```text
%LOCALAPPDATA%\Programs\KnotRoute
```

Persistent identity, configuration, service identities, CA material, logs, peer cache, and integration-state backup are kept separately under:

```text
%LOCALAPPDATA%\KnotRoute
```

This separation allows application upgrades without replacing cryptographic identities.

## Which executable to run

For normal interactive desktop use run:

```text
knotroute-desktop.exe
```

The archive also contains:

```text
knotroute.exe           CLI and overlay daemon
knotroute-service.exe   Windows Service Control Manager wrapper
knotroute-beacon.exe    Beacon/bootstrap server
knotroute-sidecar.exe   sidecar/server launcher
```

The tray controller initializes and starts `knotroute.exe` automatically when necessary.

## Tray controller

Double-click the tray icon to open the dashboard. Right-click it for:

- Open dashboard;
- Start node;
- Stop node;
- Restart node;
- Copy node `.knot` address;
- Enable/disable `.knot` system integration;
- Enable/disable launch at sign-in;
- Open the data directory;
- Exit the tray while leaving the daemon running.

The dashboard exposes v3 configuration for:

- `network_id`;
- seed peers and advertised addresses;
- Beacon/LAN/PEX discovery;
- published service identities and introduction-point count;
- direct forwards and aliases;
- circuit hop policy;
- directory replication/TTL;
- local SOCKS/HTTP gateways;
- local CA behavior.

Configuration updates are validated and atomically written before a daemon restart is requested.

## `.knot` system integration

The integration switch performs two per-user operations.

### 1. Local Root CA

KnotRoute asks for confirmation, creates the local CA if needed, and installs its public root certificate into the current user's Windows Trusted Root store.

The private CA key remains under the KnotRoute data directory. The certificate issuer rejects non-`.knot` hostnames.

When integration is disabled, KnotRoute removes its root from the current user's trusted-root store.

### 2. PAC script

The current user's `AutoConfigURL` is set to:

```text
http://127.0.0.1:8484/proxy.pac
```

The PAC script returns KnotRoute's HTTP proxy only for `.knot` names and `DIRECT` for every other destination.

Before replacement, the previous proxy-script value is saved under:

```text
%LOCALAPPDATA%\KnotRoute\proxy-state.json
```

Disabling integration restores it. If another PAC URL is already present, the tray asks before replacing it.

## Coexisting with TUN/VPN clients

The normal KnotRoute desktop mode creates no TUN/TAP adapter and does not replace the Windows IP route table.

`DIRECT` destinations from the PAC file continue through the operating system normally. If another product owns a TUN interface, that product can continue handling ordinary Internet traffic while KnotRoute handles `.knot` HTTP/HTTPS at the application-proxy layer.

Applications that ignore Windows proxy settings can explicitly use:

```text
SOCKS5  127.0.0.1:9477
HTTP    127.0.0.1:9478
```

For SOCKS5, use remote-hostname mode so `.knot` names reach KnotRoute instead of the system DNS resolver.

## HTTPS flow

For a published service-identity address:

```text
browser
  └─ TLS to local KnotRoute proxy using local Root CA
       └─ hidden-service lookup
            └─ client onion circuit
                 └─ rendezvous
                      └─ service onion circuit
                           └─ service target
```

The browser-facing TLS connection is local. The network path has its own independent end-to-end service encryption authenticated by the service identity.

## Windows Service mode

Desktop and machine-wide Service mode use different default data locations and should not bind the same ports simultaneously.

Open an elevated PowerShell window in the release directory:

```powershell
powershell -ExecutionPolicy Bypass -File .\service\install-service.ps1
```

The service scripts install under:

```text
%ProgramFiles%\KnotRoute
```

and keep service data under:

```text
%ProgramData%\KnotRoute
```

`knotroute-service.exe` is a native SCM wrapper. It starts the daemon, reports state, handles stop/shutdown controls, and terminates the child if graceful shutdown cannot complete.

Remove it with:

```powershell
powershell -ExecutionPolicy Bypass -File .\service\uninstall-service.ps1
```

Add `-RemoveData` only when machine identities should also be destroyed.

## Uninstallation

From an extracted release archive:

```powershell
powershell -ExecutionPolicy Bypass -File .\Uninstall-KnotRoute.ps1
```

The uninstaller removes system integration. By default it retains identities/configuration; use its explicit identity-removal option only when those cryptographic identities should be permanently discarded.
