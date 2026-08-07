# KnotRoute v3 protocol

This document describes the protocol implemented by KnotRoute 3.0.0. It is an implementation description, not a claim that the protocol has received independent cryptographic review.

## 1. Identities

### Node identity

Each router owns an Ed25519 key pair.

```text
node_id = SHA-256(node_ed25519_public_key)
text    = "kr_" || base32(node_id)
```

Node IDs authenticate routing peers and direct node-address services.

### Service identity

Each published hidden service owns a separate Ed25519 key pair.

```text
service_id = SHA-256(service_ed25519_public_key)
text       = "ks_" || base32(service_id)
```

A service identity is stored independently from the host node identity. Moving the service identity file preserves its canonical `.knot` address.

### Network identity

Every overlay belongs to one 32-byte network ID:

```text
text = "kn_" || base32(network_id)
```

Direct peers reject a HELLO carrying a different network ID.

## 2. `.knot` address format

Canonical labels contain one type/version byte, a 32-byte identity digest, and a two-byte checksum:

```text
payload  = type || identity[32]
checksum = SHA-256("knotroute-address-checksum-v2" || payload)[0:2]
label    = lowercase_base32(payload || checksum)
```

Types:

```text
0x01  node identity
0x02  service identity
```

Canonical forms:

```text
<node-label>.knot
<service-label>.knot
```

A direct node service may use:

```text
<service-name>.<node-label>.knot
```

A service-identity address is already one service identity and therefore accepts no service-name prefix.

## 3. Direct peer transport

Direct KnotRoute edges use TCP plus TLS 1.3.

Both sides present a self-signed Ed25519 certificate. Public WebPKI validation is intentionally not used for overlay peers. After the TLS handshake the remote node ID is derived from the certificate public key and optionally checked against a configured `expected_id` pin.

The first application frame in each direction is `HELLO`:

- protocol version;
- network ID;
- node ID;
- Ed25519 public key;
- advertised transport endpoints;
- wall-clock timestamp.

The HELLO key and node ID must match the TLS certificate. Network IDs must match and clocks must be within the allowed window.

## 4. Direct-link frame

```text
uint32_be frame_length   # includes frame type
uint8     frame_type
byte[]    payload
```

Current frame types:

| Value | Name | Purpose |
|---:|---|---|
| 1 | HELLO | peer handshake |
| 2 | LSA | signed topology advertisement |
| 3 | PACKET | routed v1/direct packet |
| 4 | PING | liveness |
| 5 | PONG | liveness |
| 6 | PEX | peer-exchange candidates |
| 7 | CIRCUIT | onion-circuit cell |

Maximum direct-link frame size is 4 MiB.

## 5. Link-state topology

Each node periodically signs an LSA containing:

- node identity and public key;
- monotonic sequence number;
- publish/expiry timestamps;
- transport endpoints;
- direct-neighbor IDs;
- direct service metadata.

An undirected graph edge `A—B` is usable only when both valid LSAs advertise each other. Routes use equal-cost shortest paths.

LSAs expose participating router identities and topology. KnotRoute v3 hides a published service's host behind introduction/rendezvous paths; it does not hide the existence of overlay routers from other participating routers.

## 6. Circuit cells

A circuit cell is:

```text
uint8     circuit_version
uint8     kind
uint64_be circuit_id
byte[]    payload
```

Kinds:

```text
CREATE
CREATED
RELAY
DESTROY
```

Circuit IDs are link-local. A relay maps an incoming `(peer, circuit_id)` to an outgoing `(peer, circuit_id)`; the same global circuit ID is not carried end-to-end.

## 7. Telescopic circuit construction

The initiator constructs a path incrementally.

For the first hop:

1. client generates ephemeral X25519 key + 32-byte nonce;
2. sends `CREATE` containing the public key and nonce;
3. relay returns its ephemeral public key + nonce in `CREATED`;
4. both derive 64 bytes through HKDF-SHA-256 and split them into forward/reverse AES-256-GCM keys.

For every additional hop, the initiator sends an onion-protected `EXTEND` command through the existing circuit. Only the current final hop learns the identity of the next directly connected hop.

The circuit-hop KDF context is:

```text
HKDF-SHA-256(
  X25519_shared,
  client_nonce || server_nonce,
  "knotroute/circuit-hop/v1",
  64
)
```

## 8. Onion layers

Relay commands are encrypted independently for each hop.

Forward direction:

```text
layer_N(payload)
layer_N-1(layer_N(payload))
...
layer_1(...)
```

Each hop removes exactly one AES-GCM layer and forwards the remaining payload on its local outgoing circuit.

Reverse traffic is wrapped by each hop on the way back. The client removes layers in hop order.

Each layer uses a strict monotonic sequence number. A sequence mismatch or authentication failure closes the circuit.

Relay commands currently include:

```text
EXTEND
EXTENDED
OPEN
OPEN_OK
OPEN_ERROR
DATA
CLOSE
```

## 9. Direct node streams

KnotRoute retains its v1 authenticated direct-node stream protocol for compatibility and explicit node-address services.

It uses:

- signed Ed25519 OPEN/ACK records;
- ephemeral X25519;
- HKDF-SHA-256;
- independent AES-256-GCM keys per direction;
- strict sequence numbers.

Published v3 service-identity addresses do not use this path for application payloads.

## 10. Service descriptors

A descriptor body contains:

- descriptor version;
- network ID;
- service ID;
- service Ed25519 public key;
- introduction-point node IDs;
- monotonically increasing revision;
- publish/expiry timestamps;
- optional application metadata.

The body is signed by the service identity. A client verifies:

1. network ID;
2. service-ID/public-key binding;
3. expiry and future-clock bound;
4. introduction-point syntax/count;
5. Ed25519 signature.

The host node identity is not present in the descriptor.

## 11. Distributed directory

The service ID is treated as a 256-bit XOR key. From the currently known overlay topology, KnotRoute selects the configured number of node IDs with the smallest XOR distance to that key and replicates the signed descriptor to them.

A lookup queries the corresponding replica set and accepts only a valid descriptor for the requested service identity. Higher revisions supersede lower revisions.

This is an XOR-replicated distributed directory; v3 does not claim to implement the complete Kademlia protocol (bucket maintenance, iterative FIND_NODE semantics, etc.).

Directory transport currently uses authenticated routed overlay packets. Directory nodes can therefore observe descriptor keys they store or answer. Service payload data does not traverse the directory protocol.

## 12. Introduction points

A published service selects several reachable routers as introduction points.

The service opens a circuit to each introduction point and sends a registration containing:

- network ID;
- service ID and public key;
- introduction-point node ID;
- expiry;
- service-identity Ed25519 signature.

The introduction point stores only the live reverse registration associated with that service identity. Registration automatically disappears when its circuit closes or expires.

## 13. Rendezvous establishment

To connect to a service identity:

1. client obtains and verifies the descriptor;
2. client chooses a reachable rendezvous router, preferably distinct from the introduction points;
3. client generates a random 32-byte rendezvous cookie and ephemeral X25519 key;
4. client opens an onion circuit to the rendezvous router and registers the cookie as `client`;
5. client opens another onion circuit to one descriptor introduction point and sends the service ID, rendezvous node, cookie, client ephemeral key, nonce, and timestamp;
6. the introduction point forwards the request over the service's existing reverse registration;
7. service opens its own onion circuit to the rendezvous router and registers the same cookie as `service`;
8. rendezvous pairs the two streams and byte-bridges them;
9. service returns a signed service acknowledgement through the paired rendezvous stream;
10. client verifies that signature against the requested service identity;
11. both endpoints derive a second end-to-end session key and application bytes begin.

The rendezvous node does not need the end-to-end service session keys.

## 14. Rendezvous end-to-end encryption

The service acknowledgement binds:

- service identity;
- service public key;
- service ephemeral X25519 public key;
- service nonce;
- rendezvous cookie;
- client ephemeral key + nonce;
- timestamp.

The endpoint KDF is:

```text
shared = X25519(local_private, remote_public)
salt   = SHA-256(client_nonce || service_nonce)
info   = "knotroute/rendezvous/v1" || service_id || cookie
keys   = HKDF-SHA-256(shared, salt, info, 64)
```

The 64 bytes are split into client-to-service and service-to-client AES-256-GCM keys.

Application data is framed, authenticated, and strictly sequenced independently in each direction.

## 15. Peer discovery

Discovery produces untrusted candidate `(node_id, endpoints)` tuples. A candidate becomes a real peer only after the normal TLS/HELLO identity checks.

### Beacon

A Beacon announcement contains:

- network ID;
- node ID and Ed25519 public key;
- normalized advertised endpoints;
- timestamp;
- Ed25519 signature.

Beacon records are soft-state and expire. A Beacon groups records by network ID, rate-limits registration by source IP, limits network capacity, and returns a bounded peer set.

A Beacon may also expose one persistent KnotRoute bootstrap relay. The relay is an ordinary overlay node and is not required after other topology edges exist.

### LAN

LAN discovery multicasts the same signed announcement model over IPv4 multicast and advertises usable non-loopback interface endpoints.

### Peer exchange

Authenticated peers periodically exchange bounded candidate lists. Candidates remain hints and are cryptographically verified when dialed.

### Cache

Recently discovered peers can be persisted and retried on a later start.

## 16. Local `.knot` browser CA

The local CA is outside the overlay wire protocol.

Each client device creates its own ECDSA P-256 root key/certificate and stores the private key locally. The HTTP CONNECT gateway may terminate browser TLS for `.knot` service-identity hostnames with a short-lived leaf certificate signed by this root.

The certificate issuer refuses non-`.knot` names in code.

After local TLS termination, the HTTP bytes travel through the hidden-service rendezvous session, which has independent end-to-end encryption.

Consequently:

```text
browser TLS trust         = local KnotRoute Root CA
network service identity  = Ed25519 service identity
network payload secrecy   = rendezvous X25519/AES-GCM session
```

These are separate trust layers.

## 17. Local gateways

SOCKS5 and HTTP/CONNECT listeners are local ingress protocols.

For a service-identity `.knot` destination they perform descriptor lookup + rendezvous connection. For a node-address destination they use explicit node routing/circuits/direct semantics as selected by the resolver.

The generated PAC script sends only `.knot` hostnames to the HTTP gateway and returns `DIRECT` for all other traffic.

## 18. Application protocols

The core transport remains a bidirectional byte stream. Optional packages add:

- framed JSON RPC;
- message-oriented datagrams;
- encrypted topic pub/sub;
- content-addressed objects;
- encrypted offline mailbox envelopes.

These are application-layer protocols and do not alter routing or hidden-service discovery.

## 19. Security limits

KnotRoute v3 does not currently provide a proof or claim of resistance to:

- global timing/volume correlation;
- a sufficiently large Sybil adversary;
- malicious-majority routing topology;
- browser/application fingerprinting;
- endpoint compromise;
- traffic confirmation attacks;
- denial of service.

The circuit implementation provides onion-style local-hop separation; the hidden-service design separates service identity from host node identity and adds rendezvous E2E encryption. These properties are useful, but they must not be confused with a mature, independently audited anonymity guarantee.
