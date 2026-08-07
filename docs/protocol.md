# KnotRoute protocol v1

This document describes the implemented protocol, not a future design.

## 1. Identity

A node owns an Ed25519 key pair. Its node ID is:

```text
raw_id  = SHA-256(ed25519_public_key)
text_id = "kr_" || lowercase_base32_without_padding(raw_id)
```

A node ID is therefore self-certifying: a received public key is accepted only when its hash equals the claimed ID.

## 2. Direct links

A direct peer connection is TCP protected by TLS 1.3. Both endpoints present a self-signed Ed25519 certificate. Certificate authority validation is intentionally not used; after the TLS handshake, KnotRoute derives the peer ID from the certificate public key and optionally compares it with the configured `expected_id` pin.

The first application frame in each direction is `HELLO`, containing protocol version, node ID, public key, advertised addresses, and wall-clock time. The HELLO identity and key must match the TLS certificate.

When duplicate links exist, the lexicographically smaller node ID prefers the outbound link and the larger ID prefers the inbound link. An existing connection is retained unless the new connection is the deterministic preferred direction.

## 3. Link frame

Every direct-link frame is:

```text
uint32_be length   # includes type byte, maximum 4 MiB
uint8     type
byte[]    payload
```

Frame types:

| Value | Name | Payload |
|---:|---|---|
| 1 | HELLO | JSON control record |
| 2 | LSA | signed JSON link-state advertisement |
| 3 | PACKET | binary overlay packet |
| 4 | PING | opaque nonce |
| 5 | PONG | echoed nonce |

## 4. Link-state advertisements

An LSA body contains:

- protocol version;
- monotonically increasing sequence number;
- creation and expiry Unix timestamps;
- node ID and Ed25519 public key;
- advertised direct-listener addresses;
- current direct-neighbor IDs;
- published service names and descriptions.

The canonical JSON encoding of the typed body is signed with Ed25519. A receiver verifies the key-to-ID binding, signature, expiry, future-clock bound, and sequence monotonicity before storing and flooding the LSA.

The route graph contains edge `A—B` only if A advertises B **and** B advertises A. Shortest paths are computed with breadth-first search because every overlay edge has equal cost in v1.

## 5. Overlay packet

The binary packet header is fixed-width:

```text
uint8  version
uint8  kind
uint8  ttl
uint8  flags
byte[32] source_node_id
byte[32] destination_node_id
byte[16] packet_id
byte[16] stream_id
uint64_be sequence
byte[] payload
```

Packet kinds:

| Value | Name |
|---:|---|
| 1 | OPEN |
| 2 | OPEN_ACK |
| 3 | DATA |
| 4 | CLOSE |
| 5 | ERROR |
| 6 | READY |

Every forwarded packet decrements TTL. Relays retain packet IDs for two minutes to suppress duplicates.

## 6. End-to-end stream handshake

### OPEN

The initiator generates an ephemeral X25519 key and 32-byte nonce. OPEN contains:

- destination service name;
- ephemeral public key;
- nonce;
- initiator Ed25519 public key;
- timestamp;
- Ed25519 signature over protocol label, stream ID, endpoint IDs, service, ephemeral key, nonce, and timestamp.

The destination verifies the signature and self-certifying source ID, checks the timestamp, resolves the named local service, applies its source-ID ACL, and connects to the local TCP target.

### OPEN_ACK

The destination generates its own ephemeral X25519 key and nonce and signs an acknowledgement bound to the stream and both endpoint IDs.

Both endpoints calculate:

```text
shared = X25519(local_ephemeral_private, remote_ephemeral_public)
salt   = SHA-256(open_nonce || ack_nonce)
info   = "knotroute/session/v1" || stream_id || initiator_id || destination_id
keys   = HKDF-SHA-256(shared, salt, info, 64 bytes)
c2s    = keys[0:32]
s2c    = keys[32:64]
```

The initiator installs stream state and sends READY. The destination does not read from its service target before READY, preventing early service banners from racing stream installation at the initiator.

## 7. DATA protection

Each direction has an independent AES-256-GCM key and sequence counter starting at zero.

The 96-bit nonce is deterministic and unique for a key:

```text
nonce_prefix = SHA-256(key || "nonce")[0:4]
nonce         = nonce_prefix || uint64_be(sequence)
```

Associated data is:

```text
stream_id || packet_source || packet_destination || uint64_be(sequence)
```

A receiver requires the exact next sequence number. Duplicate, missing, reordered, modified, or endpoint-rewritten packets close the stream.

## 8. Service model

A service maps a public overlay name to a local TCP target. Its `allow` list is enforced at the destination before the target connection is opened. Empty lists and `"*"` allow all overlay nodes.

A forward accepts local TCP connections and creates one overlay stream per connection. TCP backpressure naturally propagates through endpoint sockets and direct-link writes.

## 9. Limits

Protocol v1 does not provide onion routing, route blinding, cover traffic, padding, congestion-aware path selection, multipath streams, UDP, IP tunnelling, NAT traversal, or a distributed peer-discovery directory. These omissions are intentional and are reflected in the public threat model.

## 10. `.knot` naming

KnotRoute protocol version 1 uses a local naming layer above node IDs. The canonical node label is:

```text
payload  = uint8(address_version=1) || byte[32](node_id)
checksum = SHA-256("knotroute-address-checksum-v1" || payload)[0:2]
label    = lowercase_base32_without_padding(payload || checksum)
domain   = label || ".knot"
```

The result is 56 characters and fits inside one DNS label. A service address is:

```text
service || "." || label || ".knot"
```

The address checksum detects accidental corruption. Cryptographic authenticity is established when the endpoint presents an Ed25519 public key whose SHA-256 digest matches the node ID embedded in the address.

Human-readable aliases are local address-book entries. A portable alias record may be signed by the target node. The signature proves control of that node identity but does not make the alias globally unique.

## 11. Local proxy gateways

SOCKS5 and HTTP/CONNECT are local ingress protocols and are not forwarded as overlay control messages.

For a `.knot` destination, the gateway:

1. parses and verifies the canonical address checksum or resolves a local alias;
2. selects the explicit service prefix or a context-sensitive default service;
3. resolves the embedded node ID through the link-state route table;
4. performs the ordinary authenticated end-to-end OPEN/OPEN_ACK exchange;
5. bridges the local proxy connection to the encrypted overlay stream.

The HTTP proxy does not terminate TLS. CONNECT payloads are opaque to the local gateway after the tunnel is established.

Non-`.knot` destinations may be connected directly when `proxy.direct` is enabled. This allows a PAC file to route only `.knot` names into KnotRoute without replacing the operating system's normal route selection.
