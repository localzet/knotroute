# User guide

This guide is for a person who wants to access `.knot` services. You do not need to host a service or understand the routing protocol.

## What the client does

KnotRoute runs locally and connects to other KnotRoute nodes over the ordinary Internet. Web traffic for `.knot` destinations is sent into the overlay; ordinary Internet traffic remains on the normal operating-system path.

The desktop default does **not** create a TUN/TAP interface. It therefore normally coexists with VPN/TUN software. Applications that honor the system proxy/PAC configuration can use KnotRoute for `.knot` while other traffic continues through the normal OS routing stack.

KnotRoute has no public-Internet exit mode. It connects to services published inside the KnotRoute overlay.

## Windows: install and start

Extract the Windows release archive and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-KnotRoute.ps1
```

Then launch:

```text
knotroute-desktop.exe
```

The tray icon is the normal control surface. Double-click it to open the dashboard.

For normal interactive use, do **not** launch `knotroute-service.exe` manually. The files have these roles:

```text
knotroute-desktop.exe  tray/UI controller
knotroute.exe          CLI and overlay daemon
knotroute-service.exe  Windows Service wrapper
knotroute-beacon.exe   discovery/bootstrap server
knotroute-sidecar.exe  server/container-oriented launcher
```

## Joining a KnotRoute network

There are two common ways to receive the network parameters.

### Signed invitation file

If an operator gives you a `.knotinvite` file, initialize a local configuration first if one does not exist:

```powershell
knotroute init --config knotroute.json
```

Then import the invitation:

```powershell
knotroute invite import --config knotroute.json --file network.knotinvite
```

The invitation supplies the network ID plus bootstrap information such as Beacon URLs and configured seed peers.

A signed invitation proves which KnotRoute node created that invitation. It does **not** prove that the network itself is trustworthy and it does not grant cryptographic membership.

### Manual network settings

You may instead configure:

- `network_id`;
- one or more Beacon URLs;
- optional static seed peers.

A `network_id` only separates overlays. Treat it as an identifier, not as a password.

## Enable `.knot` browser integration on Windows

From the tray menu enable `.knot` system integration.

KnotRoute performs two per-user operations:

1. It installs the local KnotRoute Root CA in the current user's trusted-root store after confirmation.
2. It sets a PAC URL that sends `.knot` web requests to KnotRoute's local HTTP proxy and returns `DIRECT` for ordinary hosts.

Default local endpoints are:

```text
Dashboard    http://127.0.0.1:8484
SOCKS5       127.0.0.1:9477
HTTP proxy   127.0.0.1:9478
PAC          http://127.0.0.1:8484/proxy.pac
```

The CA private key stays on your device. KnotRoute's certificate issuer refuses names outside `.knot`.

If a browser does not use the Windows proxy or trusted-root settings, configure that browser explicitly or use SOCKS5/HTTP proxy mode. Do not bypass certificate warnings.

## Open a service

A canonical published service looks like:

```text
https://<service-identity>.knot/
```

A local alias may look like:

```text
https://docs.knot/
```

Aliases are local conveniences. A pretty alias is not a globally registered KnotRoute domain.

When a canonical service address is used, KnotRoute verifies the signed service descriptor and authenticates the service identity during the rendezvous session. The browser-facing TLS certificate is a separate local compatibility layer generated on your device.

## Use non-browser applications

Applications with proxy support can use:

```text
SOCKS5  127.0.0.1:9477
HTTP    127.0.0.1:9478
```

For SOCKS5, enable remote-hostname/DNS-through-proxy mode. If the application resolves the `.knot` name through system DNS before contacting SOCKS, resolution will fail because `.knot` is resolved inside KnotRoute, not public DNS.

Applications can also be written with the KnotRoute SDK and then do not require the standalone desktop client.

## Android

The standalone Android application embeds the KnotRoute core. It does not require `VpnService` or a TUN interface.

Basic flow:

1. Open the app.
2. Set the network ID and Beacon URLs in settings if you are not using the default network parameters.
3. Let the foreground service establish peers.
4. Use the **CA** button and Android's system certificate-installation dialog if HTTPS `.knot` browsing is required.
5. Open the `.knot` address in the built-in browser.

The Android application deliberately rejects TLS errors instead of ignoring them.

## Check connection health

On Windows open the dashboard and check:

- network ID;
- connected peers;
- known routes;
- active circuits;
- recent events.

From the CLI:

```powershell
knotroute doctor --config knotroute.json --probe
knotroute address --config knotroute.json
knotroute resolve --config knotroute.json <name>.knot
```

`doctor --probe` is useful when configuration loads correctly but no peers are reachable.

## Common problems

### `.knot` page does not open

Check, in order:

1. KnotRoute node is running.
2. Your `network_id` matches the target network.
3. At least one peer is connected.
4. The service descriptor has propagated.
5. System integration/PAC is enabled for the browser, or the browser is explicitly using KnotRoute's proxy.
6. For HTTPS, the local KnotRoute CA is trusted.

### Browser reports a certificate error

Do not click through it. Reinstall the local KnotRoute CA from the tray/client and verify that the browser uses the relevant OS/user trust store.

### SOCKS application cannot resolve `.knot`

Enable SOCKS remote DNS / proxy hostname resolution.

### Another VPN is running

KnotRoute normally does not compete for a TUN interface. Leave the PAC rule scoped to `.knot`; ordinary traffic remains `DIRECT` from KnotRoute's point of view and therefore follows the operating system's current routes.

## What to back up

Ordinary client identities are normally stored in the KnotRoute data directory. Back them up if retaining the same node identity matters to you.

Do not publish or share:

- node identity private files;
- local CA private keys;
- service identity private files.

A `.knotinvite`, node address, service address, network ID, or CA **public** certificate is not a private key.
