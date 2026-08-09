# KnotRoute on Windows

## Desktop installation

Extract the Windows release archive and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-KnotRoute.ps1
```

The installer places the interactive client under:

```text
%LOCALAPPDATA%\Programs\KnotRoute\KnotRoute.exe
```

Persistent identities, configuration, CA material, social state, peer cache and integration backup remain under:

```text
%LOCALAPPDATA%\KnotRoute
```

Application upgrades therefore do not replace cryptographic identities.

## Interactive client

For normal use launch only:

```text
KnotRoute.exe
```

The v4 desktop client is a single process. The KnotRoute Go runtime, messenger/social state and transport manager run in-process; Wails hosts the system WebView2 renderer for the user interface. There is no second daemon required for interactive use and the old loopback dashboard is not the desktop UI.

Main user sections are Home, Chats, Feed, Catalog, Internet, Network and Settings. Node IDs, service identities, aliases, routes, listening addresses, transport attempts and event diagnostics live under **Developers**.

Additional release tools:

```text
knotroute-cli.exe       CLI and diagnostics
knotroute-service.exe   optional Windows Service wrapper
knotroute-beacon.exe    Beacon/bootstrap server
knotroute-sidecar.exe   sidecar/server launcher
```

The UI runtime is started asynchronously so discovery or a slow transport cannot block the Windows message loop. Only changed snapshots are emitted to the frontend, and incoming state updates are deferred while the user is editing a form.

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
