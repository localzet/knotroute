# KnotRoute

**A self-hosted encrypted overlay network with self-authenticating `.knot` names, multi-hop routing, native Windows control, and no central coordinator.**

KnotRoute publishes private TCP services from laptops, home servers, VPS nodes, NAS devices, and development machines. A client reaches a service through one or more KnotRoute relays while the service payload remains end-to-end encrypted between the two endpoint nodes.

```text
browser → HTTP proxy → A → relay B → C → 127.0.0.1:8080
                         encrypted overlay
```

A service can be opened through a stable name:

```text
http://wiki.aebagbafaydqqcik...m3q.knot/
ssh.aebagbafaydqqcik...m3q.knot
```

Or through a local address-book alias:

```text
http://wiki.localzet.knot/
```

KnotRoute is narrower than a VPN and intentionally easier to deploy than an anonymity network:

- no TUN/TAP adapter in the default mode;
- no administrator privileges for normal desktop use;
- no central controller, account, hosted rendezvous service, CA, or global name registry;
- no exit nodes into the public Internet;
- TLS 1.3 on direct links and separate endpoint-to-endpoint stream encryption;
- native Windows tray controller plus an embedded management dashboard;
- SOCKS5 and HTTP/CONNECT gateways for `.knot` names;
- static Go binaries with no runtime dependency.

> KnotRoute is a private-service overlay, not a claim of Tor/I2P-grade anonymity. Relays can observe endpoint IDs, timing, packet sizes, route position, and the requested service name. See [Threat model](#threat-model).

## Windows: one-click desktop use

Download the Windows archive, extract it, and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-KnotRoute.ps1
```

The installer copies the binaries into the current user's application directory, creates shortcuts, and launches `knotroute-desktop.exe`.

The tray controller provides:

- **Start node**, **Stop node**, and **Restart node**;
- **Open dashboard**;
- **Copy `.knot` address**;
- **Enable `.knot` system integration**;
- **Start with Windows**;
- access to the configuration and log directory.

On first launch it creates the identity and configuration under:

```text
%LOCALAPPDATA%\KnotRoute
```

The node itself runs as a hidden process and continues running if the tray controller is closed. An optional true Windows Service wrapper is included under `service/` for machine-wide unattended deployments.

Detailed Windows instructions: [`docs/windows.md`](docs/windows.md).

## Coexistence with Clash, FlClashX, VPNs, and other TUN software

KnotRoute does not create a TUN interface by default. Its Windows integration installs a PAC URL that sends only `*.knot` web traffic to the local KnotRoute HTTP proxy and returns `DIRECT` for every other hostname.

Therefore, when another application owns a TUN interface, ordinary traffic still follows the operating system route table and remains under that application's control. KnotRoute handles only `.knot` destinations at the application-proxy layer.

Existing Windows proxy-script settings are saved before KnotRoute integration is enabled and restored when it is disabled.

## `.knot` addressing

Each node owns an Ed25519 key pair. Its internal node ID is the SHA-256 digest of the public key.

A canonical `.knot` label contains:

```text
version || 32-byte node ID || 2-byte checksum
```

The result is encoded with lowercase Base32 without padding and fits in one DNS label:

```text
<56-character-label>.knot
```

Service names are placed before the node address:

```text
<service>.<node-address>.knot
```

Examples:

```text
http.<node>.knot
https.<node>.knot
ssh.<node>.knot
postgres.<node>.knot
```

A bare node address uses the configured default service (`http` for ordinary HTTP and `https` for CONNECT/TLS by default).

The checksum catches typing errors. During routing and endpoint handshakes, the node proves possession of the Ed25519 private key whose public-key hash matches the embedded node ID.

Print addresses from the CLI:

```bash
knotroute address --config knotroute.json
knotroute address --config knotroute.json --service ssh
knotroute resolve --config knotroute.json ssh.example.knot
```

### Local aliases and signed alias records

Pretty names are local because KnotRoute has no global registrar and does not pretend that first-claim naming is Sybil-resistant.

```json
"aliases": [
  {
    "name": "localzet",
    "node": "kr_...",
    "description": "Localzet home node"
  }
]
```

This enables names such as:

```text
git.localzet.knot
wiki.localzet.knot
```

A node can export a signed proof that it consents to an alias:

```bash
knotroute alias export \
  --config knotroute.json \
  --name localzet \
  --description "Localzet node" \
  --out localzet.knot-alias.json
```

Import and verify it on another node:

```bash
knotroute alias import \
  --config knotroute.json \
  --file localzet.knot-alias.json
```

The signature proves ownership of the target node identity. It does not establish global uniqueness of the human-readable name.

## Local gateways

Defaults:

```text
SOCKS5       127.0.0.1:9477
HTTP proxy   127.0.0.1:9478
Dashboard    127.0.0.1:8484
PAC script   http://127.0.0.1:8484/proxy.pac
```

SOCKS5 supports CONNECT with domain-name destinations. Applications must send the hostname to the proxy rather than resolving it locally.

The HTTP gateway supports:

- ordinary HTTP proxy requests;
- HTTPS and arbitrary TCP tunnelling with `CONNECT`;
- direct fallback for non-`.knot` hosts when enabled.

KnotRoute does not intercept or terminate TLS. For HTTPS services, the destination application still needs a certificate accepted by the client for the `.knot` hostname, or the client will report its normal certificate error.

## Linux/macOS CLI quick start

Go 1.23 or newer is sufficient to build from source:

```bash
git clone https://github.com/localzet/knotroute.git
cd knotroute
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o knotroute ./cmd/knotroute
```

Initialize a node:

```bash
./knotroute init --config knotroute.json
./knotroute doctor --config knotroute.json
./knotroute run --config knotroute.json
```

This creates:

- `knotroute.json` — node configuration;
- `identity.json` — the Ed25519 private identity. Keep it private and back it up.

## Connect three nodes

Assume relay B is reachable at `relay.example.net:7447`. Add B as a seed on A and C:

```json
"peers": [
  {
    "address": "relay.example.net:7447",
    "expected_id": "kr_optional_identity_pin"
  }
]
```

Open TCP port `7447` on B. A and C may remain outbound-only. Once both direct links are established, A computes a route to C through B.

Publish SSH on C:

```json
"services": [
  {
    "name": "ssh",
    "target": "127.0.0.1:22",
    "description": "C workstation",
    "allow": ["kr_NODE_A_ID"]
  }
]
```

Create a conventional local forward on A:

```json
"forwards": [
  {
    "listen": "127.0.0.1:2222",
    "node": "kr_NODE_C_ID",
    "service": "ssh"
  }
]
```

Then:

```bash
ssh -p 2222 localhost
```

Or configure an SSH client that supports SOCKS/proxy commands and use the `.knot` service name directly.

## Configuration

```json
{
  "identity_file": "identity.json",
  "listen": ["0.0.0.0:7447"],
  "advertise": ["node.example.net:7447"],
  "peers": [
    {
      "address": "seed.example.net:7447",
      "expected_id": "kr_optional_identity_pin"
    }
  ],
  "services": [
    {
      "name": "http",
      "target": "127.0.0.1:8080",
      "description": "internal web service",
      "allow": ["kr_allowed_node_id"]
    }
  ],
  "forwards": [],
  "aliases": [
    {
      "name": "example",
      "node": "kr_remote_node_id"
    }
  ],
  "proxy": {
    "socks": "127.0.0.1:9477",
    "http": "127.0.0.1:9478",
    "direct": true,
    "default_http_service": "http",
    "default_https_service": "https"
  },
  "dashboard": "127.0.0.1:8484",
  "routing": {
    "lsa_interval": "20s",
    "lsa_ttl": "90s",
    "max_hops": 16
  }
}
```

The dashboard edits and validates this configuration. Saving triggers an in-process node restart; the tray process does not need to be restarted.

## Dashboard and local management API

The dashboard shows and manages:

- canonical `.knot` identity;
- direct peers and computed multi-hop routes;
- published services and source-node ACLs;
- static local forwards;
- local aliases;
- SOCKS5, HTTP, PAC, and routing settings;
- active streams, traffic counters, and runtime events.

Endpoints:

```text
GET  /api/health
GET  /api/status
GET  /api/config
PUT  /api/config
POST /api/reload
POST /api/shutdown
GET  /proxy.pac
```

Configuration and control endpoints accept only loopback clients and reject cross-origin browser requests. Keep the dashboard bound to loopback unless it is protected by an authenticated reverse proxy.

## Protocol and cryptography

The implemented wire protocol is documented in [`docs/protocol.md`](docs/protocol.md).

At a glance:

- direct links: mutually authenticated TLS 1.3 with self-signed Ed25519 certificates;
- node identity: `SHA-256(Ed25519 public key)`;
- routing: signed expiring link-state advertisements;
- graph safety: only mutually advertised edges are routable;
- stream handshake: ephemeral X25519 authenticated by endpoint Ed25519 signatures;
- key schedule: HKDF-SHA-256 with independent directional keys;
- payload protection: AES-256-GCM;
- relay controls: packet TTL and duplicate suppression;
- naming: versioned checksummed Base32 `.knot` labels embedding the node ID.

No custom encryption primitive is introduced.

## Threat model

KnotRoute protects against:

- passive observers between directly connected peers;
- an intermediate relay reading or modifying service payloads;
- node-ID impersonation without the corresponding Ed25519 private key;
- forged or expired topology advertisements;
- creation of a routable graph edge unless both endpoints advertise it;
- stream ciphertext modification, endpoint rewriting, duplication, and reordering.

KnotRoute does not currently hide:

- source and destination node IDs from relays on the selected path;
- the service name in the stream-open packet;
- timing, traffic volume, and packet-size patterns;
- the IP addresses of directly connected peers;
- endpoint compromise;
- global traffic analysis, Sybil attacks, censorship, or denial of service.

It is suitable for private service routing, self-hosting, and networking experiments. It is not suitable where personal safety depends on strong anonymity against a global adversary.

## Commands

```text
knotroute init     [--config knotroute.json] [--force]
knotroute run      [--config knotroute.json]
knotroute id       [--config knotroute.json]
knotroute address  [--config knotroute.json] [--service name]
knotroute resolve  [--config knotroute.json] <name.knot>
knotroute alias export --config knotroute.json --name name [--out record.json]
knotroute alias import --config knotroute.json --file record.json
knotroute doctor   [--config knotroute.json] [--probe]
knotroute version
```

## Tests

```bash
go test ./...
go test -race ./...
go vet ./...
```

Integration tests create three real nodes and verify:

- a two-hop A → B → C route;
- endpoint-authenticated encrypted byte streams;
- HTTP access through a canonical `.knot` name and relay;
- SOCKS5 access through `service.<node>.knot` and relay.

## Release targets

`make release` produces archives for:

- Linux amd64/arm64;
- Windows amd64/arm64;
- macOS amd64/arm64.

Windows archives include the daemon, tray controller, Service wrapper, current-user installer, service scripts, and documentation.

## License

Apache License 2.0. See [`LICENSE`](LICENSE).
