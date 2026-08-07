# KnotRoute

**A self-hosted, encrypted, multi-hop overlay network for publishing private TCP services.**

KnotRoute turns a group of machines into a routed service network. A service can live on a laptop, home server, VPS, NAS, or development machine; another node reaches it through one or more KnotRoute peers using a stable cryptographic node ID. Intermediate relays forward encrypted packets but cannot read the service payload.

It is deliberately narrower than a VPN and more deployable than an anonymity network:

- no virtual network adapter or administrator privileges;
- no central controller, account system, PKI, or hosted rendezvous service;
- no exit nodes into the public Internet;
- no dependency on DNS for identity;
- one static native binary with an embedded dashboard;
- the complete data plane builds with the Go standard library only.

> KnotRoute is a private-service overlay, not a claim of I2P/Tor-grade anonymity. Relays can observe node IDs, timing, packet sizes, and service-opening metadata. See [Threat model](#threat-model).

## What it does

1. Every node creates an Ed25519 identity. Its address is `kr_` plus the SHA-256 hash of its public key.
2. Direct peers authenticate each other with self-signed Ed25519 certificates over TLS 1.3.
3. Nodes flood signed link-state advertisements. A route only uses an edge when **both endpoints advertise it**, so one node cannot invent a usable link on its own.
4. The overlay computes shortest multi-hop paths and relays binary packets with hop limits and duplicate suppression.
5. A stream endpoint performs an independent X25519 handshake, derives directional keys with HKDF-SHA-256, and encrypts data with AES-256-GCM. Relays see ciphertext.
6. A node publishes named local TCP services; another node creates a local TCP forward to any reachable service.

Typical uses:

- reach SSH, PostgreSQL, Redis, Home Assistant, a dev server, or an internal HTTP service without exposing it publicly;
- bridge machines that cannot all connect directly, using one or more ordinary nodes as relays;
- create a small private service fabric across home, office, VPS, and travel devices;
- give temporary collaborators access to one named service rather than an entire subnet;
- run reproducible multi-hop networking experiments without a central control plane.

## Quick start

### 1. Build

Go 1.23 or newer is sufficient. The embedded dashboard is already compiled, so Node.js is not required for a normal build.

```bash
git clone https://github.com/localzet/knotroute.git
cd knotroute
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o knotroute ./cmd/knotroute
```

On Windows PowerShell:

```powershell
go test -race ./...
$env:CGO_ENABLED = "0"
go build -trimpath -o knotroute.exe ./cmd/knotroute
```

### 2. Initialize each node

```bash
./knotroute init --config knotroute.json
```

This writes:

- `knotroute.json` — non-secret configuration;
- `identity.json` — the node's Ed25519 private key; keep it private and back it up.

Print a node ID at any time:

```bash
./knotroute id --config knotroute.json
```

### 3. Connect nodes

Assume node **B** is reachable at `relay.example.net:7447`. Add it to node A and node C:

```json
"peers": [
  { "address": "relay.example.net:7447" }
]
```

For identity pinning, add B's ID:

```json
"peers": [
  {
    "address": "relay.example.net:7447",
    "expected_id": "kr_..."
  }
]
```

Open TCP port `7447` on B. A and C only need outbound access to B. Once both links are up, A can route to C through B.

### 4. Publish a service on C

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

An omitted or empty `allow` list permits every node in the overlay. Use explicit IDs for sensitive services.

### 5. Create a local forward on A

```json
"forwards": [
  {
    "listen": "127.0.0.1:2222",
    "node": "kr_NODE_C_ID",
    "service": "ssh"
  }
]
```

Start the nodes:

```bash
./knotroute doctor --config knotroute.json
./knotroute run --config knotroute.json
```

Then on A:

```bash
ssh -p 2222 localhost
```

The TCP stream travels A → B → C. B forwards it but does not possess the end-to-end stream keys.

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
      "name": "web",
      "target": "127.0.0.1:8080",
      "description": "internal dashboard",
      "allow": ["kr_allowed_node_id"]
    }
  ],
  "forwards": [
    {
      "listen": "127.0.0.1:18080",
      "node": "kr_remote_node_id",
      "service": "web"
    }
  ],
  "dashboard": "127.0.0.1:8484",
  "routing": {
    "lsa_interval": "20s",
    "lsa_ttl": "90s",
    "max_hops": 16
  }
}
```

| Field | Meaning |
|---|---|
| `identity_file` | Ed25519 identity path, resolved relative to the config file. |
| `listen` | Direct-peer TCP listeners. Port `0` is useful in tests. |
| `advertise` | Addresses placed in node metadata. If omitted, bound addresses are advertised. |
| `peers` | Persistent outbound seed connections with exponential reconnect. |
| `expected_id` | Optional self-certifying identity pin for a seed. |
| `services` | Named local TCP targets published into the overlay. |
| `allow` | Source node IDs allowed to open a service; `"*"` permits all. |
| `forwards` | Local TCP listeners connected to a remote named service. |
| `dashboard` | Read-only local dashboard and status API; empty disables it. |
| `lsa_interval` | Signed topology refresh interval. |
| `lsa_ttl` | Advertisement lifetime; must be at least twice the interval. |
| `max_hops` | Packet TTL, from 2 to 64. |

## Dashboard and API

The default dashboard is `http://127.0.0.1:8484`. It shows:

- direct peers and connection direction;
- all computed routes and complete paths;
- services announced by reachable nodes;
- local services and forwards;
- active streams and traffic counters;
- a bounded runtime event log.

Read-only endpoints:

```text
GET /api/health
GET /api/status
```

The dashboard binds to loopback by default. Do not expose it publicly without a reverse proxy and authentication.

## Commands

```text
knotroute init   [--config knotroute.json] [--force]
knotroute run    [--config knotroute.json]
knotroute id     [--config knotroute.json]
knotroute doctor [--config knotroute.json] [--probe]
knotroute version
```

`doctor` validates the configuration and identity, verifies listener availability, and optionally probes configured service targets.

## Protocol and cryptography

The wire design is documented in [`docs/protocol.md`](docs/protocol.md).

At a glance:

- direct link: TLS 1.3 with Ed25519 certificates;
- node identity: `SHA-256(Ed25519 public key)`;
- topology: signed, expiring link-state advertisements;
- graph safety: only mutually advertised edges are routable;
- endpoint handshake: ephemeral X25519 plus Ed25519 signatures;
- key schedule: HKDF-SHA-256, separate client→service and service→client keys;
- payload: AES-256-GCM with stream ID, endpoints, and sequence number as associated data;
- relay controls: TTL and packet-ID duplicate cache;
- transport payload: length-prefixed binary frames; JSON is limited to infrequent control messages.

No custom encryption algorithm is invented. The small local HKDF routine is a direct RFC 5869 extract-and-expand implementation used to avoid a runtime dependency.

## Threat model

KnotRoute protects against:

- passive observers between directly connected peers;
- an intermediate KnotRoute relay reading or modifying service payloads;
- node-ID impersonation without the corresponding Ed25519 private key;
- tampering with or replaying older link-state advertisements;
- a node creating a routable graph edge unless the other endpoint also advertises that edge;
- ciphertext modification and stream packet reordering.

KnotRoute does **not** currently hide:

- source and destination node IDs from relays on the selected path;
- service name in the stream-open control packet;
- timing, volume, and packet-size patterns;
- a malicious destination service reading application data;
- endpoint compromise or theft of `identity.json`;
- global traffic analysis, Sybil attacks, censorship, or denial of service;
- the IP addresses of directly connected peers.

It is therefore suitable for private service routing and experimentation, not for users whose safety depends on strong anonymity against a global adversary.

## Operational notes

- A node behind NAT can operate outbound-only by keeping `listen` on loopback or firewalling it and configuring one or more reachable seeds.
- Relays require only the KnotRoute listener; they do not require any published service.
- Multiple configured seeds improve availability. Routes update when links or signed LSAs change.
- Identity loss changes the node ID. Back up `identity.json` securely.
- Use application-layer authentication too. KnotRoute protects transport; it does not replace SSH keys, database credentials, or HTTP authentication.

## Native distribution

`make release` creates static archives for:

- Linux amd64/arm64;
- Windows amd64/arm64;
- macOS amd64/arm64.

The process is defined in `scripts/build-release.sh` and `scripts/build-release.ps1`. A systemd unit, Windows service installer, Dockerfile, and Compose example are included under `packaging/`.

## Development

```bash
make test       # unit + integration tests
make race       # race detector, including A -> B -> C stream test
make vet
make build
make ui         # rebuild embedded TypeScript dashboard
```

The integration test creates three real nodes, waits for a two-hop route, performs the endpoint key exchange, and sends a byte stream through the relay.

## License

Apache License 2.0. See [`LICENSE`](LICENSE).
