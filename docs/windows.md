# KnotRoute on Windows

## Desktop installation

Extract the release archive and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-KnotRoute.ps1
```

The installer is per-user and does not require elevation. It copies:

```text
%LOCALAPPDATA%\Programs\KnotRoute\knotroute.exe
%LOCALAPPDATA%\Programs\KnotRoute\knotroute-desktop.exe
```

Identity, configuration, logs, and proxy-setting backup are stored separately:

```text
%LOCALAPPDATA%\KnotRoute
```

The separation allows an application upgrade without deleting the node identity.

## Tray controller

Double-click the tray icon to open the dashboard. Right-click it for:

- Open dashboard;
- Start node;
- Stop node;
- Restart node;
- Copy `.knot` address;
- Enable or disable `.knot` system integration;
- enable or disable launch at sign-in;
- open the data directory;
- exit the tray while leaving the node running.

The desktop controller automatically initializes a configuration on first start and starts the hidden daemon process. The daemon can be stopped from either the tray or dashboard.

## `.knot` system integration

The integration switch sets the current user's `AutoConfigURL` to:

```text
http://127.0.0.1:8484/proxy.pac
```

The PAC script returns the KnotRoute HTTP proxy only for `.knot` hostnames and returns `DIRECT` for all other destinations.

Before changing the setting, the controller records the previous script URL in:

```text
%LOCALAPPDATA%\KnotRoute\proxy-state.json
```

Disabling integration restores the previous value. If another proxy script is already configured, KnotRoute asks before replacing it.

A separate TUN-based product can continue routing normal `DIRECT` traffic. KnotRoute does not create, modify, or claim a TUN adapter.

Applications that ignore Windows proxy settings can use:

```text
SOCKS5: 127.0.0.1:9477
HTTP:   127.0.0.1:9478
```

Use remote-hostname SOCKS mode so the application sends `.knot` names to KnotRoute.

## Windows Service mode

Desktop and Service mode use different default data locations and should not be run simultaneously with conflicting listener ports.

Open an elevated PowerShell window in the release directory:

```powershell
powershell -ExecutionPolicy Bypass -File .\service\install-service.ps1
```

The script installs:

```text
%ProgramFiles%\KnotRoute\knotroute.exe
%ProgramFiles%\KnotRoute\knotroute-service.exe
```

Service data is stored in:

```text
%ProgramData%\KnotRoute
```

The `knotroute-service.exe` binary is a native Service Control Manager wrapper. It starts the KnotRoute daemon, reports service state, handles stop and shutdown controls, and terminates the child process if graceful shutdown does not complete.

Remove it with:

```powershell
powershell -ExecutionPolicy Bypass -File .\service\uninstall-service.ps1
```

Add `-RemoveData` to delete the machine identity and configuration too.

## Uninstallation

From the extracted release directory:

```powershell
powershell -ExecutionPolicy Bypass -File .\Uninstall-KnotRoute.ps1
```

By default, the identity and configuration are retained. Use `-RemoveIdentity` only when the node identity should be permanently deleted.
