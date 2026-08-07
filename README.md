# KnotRoute

**KnotRoute is a self-hosted encrypted overlay network for private services, self-authenticating `.knot` names, onion-style circuits, rendezvous services, automatic peer discovery, desktop/Android clients, and embeddable application SDKs.**

KnotRoute uses the ordinary Internet only as transport between participating nodes. Services do not need public DNS names or public HTTP ports, and the network has no exit-node mode into the public Internet.

```text
                                  KnotRoute overlay

 browser/app ─ local gateway ─ client circuit ─ rendezvous ─ service circuit ─ private service
                                  │      │                     │      │
                                relay  relay                 relay  relay

                         signed service descriptor
                                  │
                       replicated XOR directory
```

A published service owns a stable cryptographic address independent of the machine that currently hosts it:

```text
https://aib6r4...q3.knot/
```

The same service identity can be moved to another KnotRoute node by moving its service identity file.

> KnotRoute v3 provides onion-style hop encryption and hidden-service-style introduction/rendezvous paths, but it has **not** received the years of public cryptographic review, traffic-analysis research, or operational hardening behind Tor/I2P. Do not market it as providing Tor-equivalent anonymity. See [Security model](#security-model).

## Documentation by role

- [End-user guide](docs/user-guide.md) — install a client, join a network, enable `.knot` browser integration, and troubleshoot access.
- [Service operator guide](docs/service-operator.md) — publish sites/TCP services, manage stable service identities, migration, and application-layer access control.
- [Server operator guide](docs/server-operator.md) — relays, Docker sidecars, Traefik coexistence, monitoring, backups, and upgrades.
- [Network operator guide](docs/network-operator.md) — create an independent network, deploy Beacons/bootstrap relays, distribute invitations, monitor topology, and handle incidents.
- [SDK guide](docs/sdk.md) — embed client/server KnotRoute into applications.
- [Documentation index](docs/README.md) — all guides and protocol references.

> `network_id` isolates overlays but is **not** a membership credential or password. Current v3 does not provide network-wide admission/revocation. Published hidden services also require application-layer authentication when access must be restricted.

## What v3 contains

- Ed25519 node identities and independent Ed25519 **service identities**.
- Versioned checksummed `.knot` node and service addresses.
- TLS 1.3 between directly connected peers.
- Signed link-state topology advertisements.
- Telescopically constructed multi-hop circuits using per-hop X25519 keys and AEAD layers.
- Hidden services using introduction points and a separately selected rendezvous node.
- Additional end-to-end X25519 + AES-256-GCM encryption between the client and service across the rendezvous path.
- Signed service descriptors replicated to XOR-nearest directory nodes.
- Multiple isolated networks selected by `network_id`.
- Automatic peer discovery through multiple Beacon URLs, LAN multicast, peer exchange, and persistent peer cache.
- A standalone **KnotRoute Beacon** container that can also provide a bootstrap relay.
- SOCKS5 and HTTP/CONNECT local gateways.
- A local KnotRoute Root CA for normal `https://*.knot` browser UX.
- Native Windows tray controller, per-user system integration, autostart, and optional Windows Service wrapper.
- Android browser/client backed by the same Go networking core through a generated AAR.
- Embeddable Go client and server SDKs.
- Gomobile binding for Android applications that want KnotRoute without requiring the standalone client app.
- Docker sidecar for publishing existing containers as `.knot` services without exposing their application port on the host.
- Application primitives for RPC, message-oriented datagrams, encrypted topic pub/sub, content-addressed objects, and encrypted offline mailboxes.
- Signed network invitation bundles.

## Addresses

KnotRoute has two canonical address types.

### Node address

A node address identifies one KnotRoute router:

```text
<node-label>.knot
ssh.<node-label>.knot
```

The label contains a type/version byte, the SHA-256 node ID derived from the node's Ed25519 public key, and a checksum.

Node addresses are useful for diagnostics and direct services. A relay can learn the destination node of a direct node-address connection, so they are **not** the preferred v3 hidden-service address.

### Service address

A published v3 service owns an independent Ed25519 identity:

```text
<service-label>.knot
```

The service label is derived from the service public key, not the host node. A descriptor tells clients only which introduction points currently accept introductions for that service. The hosting node identity is not part of the address or descriptor.

Print addresses:

```bash
knotroute address --config knotroute.json
knotroute address --config knotroute.json --service web
```

A service configured with `"publish": true` prints its independent service-identity address. A non-published service prints the legacy/direct `service.<node>.knot` form.

### Human-readable aliases

Aliases are intentionally local; KnotRoute has no global registrar that could honestly guarantee globally unique pretty names without introducing another trust/consensus system.

```json
{
  "aliases": [
    {
      "name": "docs",
      "service_id": "ks_...",
      "description": "Documentation service"
    }
  ]
}
```

Then:

```text
https://docs.knot/
```

Node aliases can also be exported/imported as signed consent records:

```bash
knotroute alias export --config knotroute.json --name edge --out edge.knot-alias.json
knotroute alias import --config knotroute.json --file edge.knot-alias.json
```

## Hidden-service connection flow

For a published service, the normal v3 path is:

1. The service keeps reverse circuits open to several introduction points.
2. It signs a descriptor containing its service public key, introduction-point IDs, revision, expiration, and optional metadata.
3. The descriptor is replicated to several XOR-nearest directory nodes.
4. A client looks up and verifies the descriptor.
5. The client selects a rendezvous node and opens an onion circuit to it.
6. The client sends an introduction request through a different onion circuit to an introduction point.
7. The service independently opens its own circuit to the rendezvous node.
8. Client and service authenticate the service identity and derive an additional end-to-end session key.
9. The rendezvous node bridges ciphertext. It does not receive the service payload key.

This is deliberately different from direct node-address routing: the service address does not disclose which node hosts the service.

## Circuits

A client circuit is built one hop at a time. Each hop receives its own ephemeral X25519 handshake and separate forward/reverse AEAD keys.

```text
client
  │ layer A(layer B(layer C(payload)))
  ▼
relay A  -> removes/adds only layer A
  ▼
relay B  -> removes/adds only layer B
  ▼
relay C  -> removes/adds only layer C
```

A relay stores only the adjacent incoming/outgoing circuit identifiers and peers for that circuit. It does not receive the complete client-selected path as one routing header.

`privacy.circuit_hops` sets the desired minimum hop count where the currently known topology allows it. KnotRoute does not invent unreachable hops merely to satisfy the configured number.

## Automatic peer discovery

Discovery sources are additive:

- **Beacon** — signed soft-state peer announcements over HTTP/HTTPS;
- **LAN** — signed IPv4 multicast announcements on the local network;
- **PEX** — peer candidates exchanged over already authenticated KnotRoute links;
- **peer cache** — recently verified candidates persisted locally.

Every candidate is only a hint. The real KnotRoute TLS/HELLO handshake verifies the peer's cryptographic identity and `network_id`, so a Beacon or PEX sender cannot make a client accept an endpoint as the wrong node identity.

A client with no publicly reachable listener may query Beacons without advertising a useless loopback/wildcard endpoint.

### KnotRoute Beacon

Beacon is intentionally not a service directory and not a traffic relay requirement. It stores only short-lived signed peer announcements grouped by `network_id`.

The provided container can additionally run one normal KnotRoute bootstrap relay so a brand-new outbound-only client immediately has a first overlay edge.

```bash
docker compose -f compose.beacon.yaml up -d --build
```

Relevant ports in the example:

```text
8080/tcp  Beacon HTTP API
7447/tcp  optional bootstrap KnotRoute relay
```

Use several independent Beacon URLs if availability matters. Once nodes have discovered peers, the overlay can continue operating without the Beacon as long as the topology remains connected.

## Independent networks and invitations

Every direct peer handshake includes `network_id`; nodes with different IDs refuse each other.

Generate a fresh network ID:

```bash
knotroute network create
```

Export the current network bootstrap information as a signed invitation:

```bash
knotroute invite export --config knotroute.json --out network.knotinvite
```

Import it:

```bash
knotroute invite import --config knotroute.json --file network.knotinvite
```

The invite authenticates who created the bundle and transports the network ID, Beacon URLs, and configured seeds. It is a bootstrap object, **not** a global naming authority or a cryptographic membership/authorization system.

## Browser integration and local CA

Defaults:

```text
SOCKS5       127.0.0.1:9477
HTTP proxy   127.0.0.1:9478
Dashboard    127.0.0.1:8484
PAC          http://127.0.0.1:8484/proxy.pac
```

The PAC script sends only `.knot` web destinations to KnotRoute and returns `DIRECT` for everything else. This means KnotRoute can coexist with a separate VPN/TUN product: KnotRoute does not create a TUN device in its default desktop mode.

### HTTPS

When `ca.enabled` and `ca.intercept_https` are enabled, the local HTTP proxy terminates browser TLS for every resolvable `.knot` hostname: canonical service identities, node-bound service names such as `web.<node>.knot`, and configured local aliases. It presents a short-lived certificate issued by a per-device KnotRoute Root CA, then sends plaintext HTTP bytes into the already authenticated/encrypted KnotRoute stream or rendezvous session.

The CA private key never leaves the local device, and certificate issuance explicitly refuses non-`.knot` names.

This is a local browser-compatibility mechanism, not a network-wide certificate authority.

## Windows desktop client

Build/download the Windows archive and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-KnotRoute.ps1
```

Then launch:

```text
knotroute-desktop.exe
```

The tray menu provides:

- Start / stop / restart node;
- Open dashboard;
- Copy node `.knot` address;
- Enable / disable `.knot` system integration;
- Start with Windows;
- Open the KnotRoute data directory.

Enabling system integration explicitly asks before:

1. installing the **per-user** KnotRoute Root CA into the Windows Trusted Root store;
2. setting the current user's PAC URL.

The previous PAC setting is backed up and restored when integration is disabled. The Root CA is removed at the same time.

The ordinary desktop setup does not require a TUN adapter. An optional `knotroute-service.exe` wrapper is included for unattended Windows Service deployments.

See [`docs/windows.md`](docs/windows.md).

## Android client

The Android project is under [`android/`](android/). It embeds the Go core generated from [`mobile/knotmobile`](mobile/knotmobile) with `gomobile bind`.

The app:

- runs the overlay in a foreground service;
- uses AndroidX WebKit `ProxyController` so its WebView sends traffic through the process-local KnotRoute proxy;
- can import the local KnotRoute Root CA through Android's normal certificate installation UI;
- explicitly cancels WebView TLS errors rather than bypassing certificate validation;
- requires no TUN/VPN service.

Build:

```bash
./scripts/build-android.sh
```

Requirements are documented in [`docs/android.md`](docs/android.md). GitHub Actions also builds the debug APK, unsigned release APK, and reusable AAR.

## Embedding KnotRoute into applications

### Go client

```go
client, err := knotclient.New(knotclient.Options{
    DataDir:      "./knot-data",
    Beacons:      []string{"https://beacon.example"},
    CircuitHops:  3,
    PeerExchange: true,
})
if err != nil {
    log.Fatal(err)
}
if err := client.Start(ctx); err != nil {
    log.Fatal(err)
}
defer client.Close()

conn, err := client.Dial(ctx, "<service-address>.knot")
```

Import:

```go
import "github.com/localzet/knotroute/pkg/knotclient"
```

`knotclient` can also return an `http.Client` for plain HTTP `.knot` requests and can expose local HTTP/SOCKS gateways if the embedding application wants proxy semantics.

### Go server

`pkg/knotserver` lets a process publish an in-process handler as an independent hidden service without opening an application port externally.

```go
host, err := knotserver.New(knotserver.Options{
    DataDir: "./knot-server",
    Beacons: []string{"https://beacon.example"},
    Services: []knotserver.Service{{
        Name: "rpc",
        Handler: knotserver.HandlerFunc(func(conn net.Conn) {
            // Serve the application protocol on conn.
        }),
    }},
})
```

### Android AAR

The release workflow produces:

```text
knotroute-client_3.0.1_android.aar
```

An Android application can construct the embedded client, call `Start()`, and use `OpenForward()` to obtain a loopback TCP port connected to a `.knot` service. The standalone KnotRoute Android app is therefore optional for applications that embed the AAR.

See [`docs/sdk.md`](docs/sdk.md).

## Application primitives

`pkg/knotapp` contains optional protocols built on top of a KnotRoute stream:

- `RPC` — request/response JSON RPC primitive;
- `DATAGRAM` — one framed message per short-lived stream;
- `PUBSUB` — topic-secret encrypted messages through an untrusted broker;
- `OBJECT` — SHA-256 content-addressed object put/get with integrity verification.

`pkg/knotmailbox` provides an offline mailbox primitive where the relay stores encrypted envelopes, recipients authenticate fetch/ack operations, and plaintext remains end-to-end protected.

These packages are independent building blocks; KnotRoute does not force applications into a specific application protocol.

## Docker sidecar

A service container does not need a host port merely to become a KnotRoute service.

```yaml
services:
  app:
    image: nginx:alpine
    networks: [private]

  knotroute:
    build:
      context: .
      dockerfile: Dockerfile.sidecar
    restart: unless-stopped
    networks: [private]
    volumes:
      - knotroute-data:/data
    environment:
      KNOTROUTE_BEACONS: "https://beacon.example"
      KNOTROUTE_SERVICE_NAME: "web"
      KNOTROUTE_SERVICE_TARGET: "app:80"
```

The application can simultaneously participate in any normal reverse-proxy setup; KnotRoute only needs TCP reachability to the chosen target inside a shared Docker network.

Multiple services can be provided through `KNOTROUTE_SERVICES_JSON`.

See [`docs/docker.md`](docs/docker.md) and [`examples/docker-private-site/`](examples/docker-private-site/).

## CLI quick start

Requires Go 1.23+ to build from source.

```bash
go test ./...
go build -trimpath -o knotroute ./cmd/knotroute
./knotroute init --config knotroute.json
./knotroute doctor --config knotroute.json
./knotroute run --config knotroute.json
```

Useful commands:

```text
knotroute init
knotroute run
knotroute id
knotroute address
knotroute resolve
knotroute alias export|import
knotroute doctor
knotroute ca init|path|fingerprint|install|uninstall
knotroute network create
knotroute invite export|import
knotroute version
```

## Example published service

```json
{
  "name": "web",
  "target": "127.0.0.1:8080",
  "publish": true,
  "intro_count": 3,
  "allow": ["*"],
  "metadata": {
    "protocol": "http"
  }
}
```

The first start creates a persistent service identity under the configured service identity path. Its canonical address can then be printed with:

```bash
knotroute address --config knotroute.json --service web
```

Back up the service identity file if the `.knot` address must survive host migration.

## Release builds

### Linux/macOS

```bash
VERSION=3.0.1 ./scripts/build-release.sh
```

### Windows PowerShell

```powershell
$env:VERSION = "3.0.1"
.\scripts\build-release.ps1
```

The scripts create cross-platform desktop/server archives in `dist/`. Android is built separately because it requires the Android SDK and gomobile:

```bash
./scripts/build-android.sh
```

Pushing a Git tag matching `v*` runs the GitHub release workflow, builds Android plus desktop/server archives, recalculates SHA-256 checksums, and attaches the artifacts to the GitHub Release.

## Docker images

Build the generic node:

```bash
docker build -t knotroute:3.0.1 .
```

Build the application sidecar:

```bash
docker build -f Dockerfile.sidecar -t knotroute-sidecar:3.0.1 .
```

Build Beacon/bootstrap relay:

```bash
docker build -f Dockerfile.beacon -t knotroute-beacon:3.0.1 .
```

All runtime images are `scratch`-based static binaries (the sidecar additionally contains CA certificates for outbound HTTPS Beacon access).

## Security model

KnotRoute v3 is designed to provide:

- cryptographic node identity;
- encrypted authenticated peer links;
- path-local onion circuit layers;
- service identities independent of host nodes;
- signed descriptors;
- hidden-service introduction/rendezvous paths;
- end-to-end payload confidentiality between hidden-service client and service;
- no public-Internet exit mode;
- no requirement for a central service registry.

It does **not** currently claim resistance to every anonymity attack. In particular, operators must assume exposure to some combination of:

- timing and volume correlation;
- global/passive traffic observation;
- Sybil-heavy overlays;
- malicious introduction/rendezvous/directory nodes;
- topology fingerprinting;
- denial of service and route manipulation by participating nodes;
- application-layer identity leaks, cookies, browser fingerprinting, or identifying content.

The local CA also expands the trust of the machine on which it is installed. KnotRoute deliberately keeps its CA private key local and refuses non-`.knot` certificate issuance, but compromise of that local key would allow forged `.knot` certificates on that device until the CA is removed/replaced.

Read [`SECURITY.md`](SECURITY.md) before deploying KnotRoute for sensitive workloads.

## Tests

The repository includes unit and integration tests for:

- identities and address checksums;
- signed descriptors and invitations;
- Beacon registration and verification;
- link-state routing;
- direct encrypted streams;
- SOCKS5/HTTP proxy paths;
- multi-hop circuit construction;
- published service lookup, introduction, rendezvous, and encrypted payload exchange;
- local CA constraints;
- RPC/datagram/pubsub/object protocols;
- encrypted offline mailbox operations.

Run:

```bash
go test ./...
go test -race ./...
go vet ./...
```

CI runs Go tests on Linux, Windows, and macOS, and has a separate Android build job.

## Repository layout

```text
cmd/knotroute             CLI/node
cmd/knotroute-desktop     native Windows tray controller
cmd/knotroute-service     Windows SCM wrapper
cmd/knotroute-beacon      peer-discovery/bootstrap service
cmd/knotroute-sidecar     Docker publishing sidecar
internal/overlay          routing, circuits, directory, rendezvous
internal/discovery        Beacon, LAN, PEX/cache discovery
internal/certauth         local .knot CA
pkg/knotclient            embeddable Go client
pkg/knotserver            embeddable Go service host
pkg/knotapp               application primitives
pkg/knotmailbox           offline encrypted mailbox primitive
mobile/knotmobile         gomobile-facing API
android/                   Android application
web/                       dashboard source
packaging/                 OS integration
```

## License

MIT. See [`LICENSE`](LICENSE).
