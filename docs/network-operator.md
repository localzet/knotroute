# Network operator guide

This guide is for the operator responsible for an independent KnotRoute overlay as a whole: its network ID, bootstrap infrastructure, discovery endpoints, baseline relays, onboarding material, upgrades, and incident response.

KnotRoute intentionally does not require a central naming or traffic authority. A network operator therefore manages **bootstrap and operational policy**, not every connection or service descriptor.

## 1. Create an independent network ID

Do not use the built-in default network ID when you want an isolated deployment.

Generate one:

```bash
knotroute network create
```

Example output:

```text
kn_...
```

Use the exact same value on:

- Beacons/bootstrap relays;
- general relays;
- service hosts/sidecars;
- desktop clients;
- Android clients;
- embedded SDK applications.

### Security property of `network_id`

`network_id` is an isolation/routing namespace, **not a secret admission credential**. The peer handshake rejects nodes from another network ID, but anyone who knows the ID and can reach a bootstrap path may run compatible software and attempt to participate.

Do not use secrecy of `network_id` as your only access-control mechanism.

Current v3 has no network-wide membership PKI, allow-list authority, or revocation list. If strict cryptographic admission control is required, it must be added at the protocol/deployment layer rather than assumed to exist.

## 2. Deploy more than one Beacon

A practical baseline is two or three independently hosted Beacons. Each uses the same `network_id` but has its own relay identity and persistent `/data` volume.

Example Beacon A:

```yaml
services:
  beacon:
    image: knotroute-beacon:3.0.0
    restart: unless-stopped
    ports:
      - "7447:7447"
    volumes:
      - beacon-data:/data
    environment:
      KNOTROUTE_NETWORK_ID: "kn_..."
      KNOTROUTE_BEACON_LISTEN: "0.0.0.0:8080"
      KNOTROUTE_BEACON_RELAY: "true"
      KNOTROUTE_BEACON_RELAY_LISTEN: "0.0.0.0:7447"
      KNOTROUTE_BEACON_RELAY_ADVERTISE: "relay-a.example.net:7447"

volumes:
  beacon-data:
```

Put the HTTP Beacon endpoint behind HTTPS, for example:

```text
https://beacon-a.example.net
https://beacon-b.example.net
https://beacon-c.example.net
```

Expose `7447/tcp` independently for the bundled relay.

Clients may list all Beacon URLs. Results are additive; no single Beacon is authoritative.

## 3. Run baseline relays

Beacons can provide initial relay edges, but a healthy overlay should not depend only on them.

Run several always-on general KnotRoute nodes with:

- stable identities;
- reachable TCP `advertise` endpoints;
- no application service requirement;
- discovery enabled or explicit stable peers.

The v3 default circuit target is three hops and the default directory replication target is five nodes. These values do not create nonexistent topology: a small network simply has less path diversity and fewer available replicas.

For an operationally useful independent deployment, five or more stable routing-capable nodes is a sensible starting topology, with additional transient/client nodes joining around them. This is an availability guideline, not an anonymity guarantee.

## 4. Configure discovery policy

KnotRoute combines:

```text
Beacon
LAN multicast
Peer Exchange (PEX)
persistent peer cache
static seed peers
```

Recommended server/desktop defaults for a distributed network:

```json
{
  "discovery": {
    "enabled": true,
    "beacons": [
      "https://beacon-a.example.net",
      "https://beacon-b.example.net"
    ],
    "lan": true,
    "peer_exchange": true,
    "cache_file": "peers.json",
    "interval": "30s"
  }
}
```

Disable LAN where local multicast discovery is undesirable. Keep PEX and peer cache enabled unless there is a specific topology-control reason not to.

Beacon is a discovery hint. Peer identity and `network_id` are verified again during the actual KnotRoute handshake.

## 5. Create onboarding material

Create a configured KnotRoute node using the target network ID and bootstrap list, then export:

```bash
knotroute invite export --config knotroute.json --out network.knotinvite
```

Distribute the `.knotinvite` through a channel appropriate to your deployment.

A recipient imports it with:

```bash
knotroute invite import --config knotroute.json --file network.knotinvite
```

The invitation contains bootstrap configuration and a creator signature. It is not a membership certificate and cannot revoke a participant later.

For Android v3, the current standalone UI accepts network ID and Beacon URLs directly. Treat the invitation as the canonical desktop/CLI bootstrap artifact unless the Android UI gains invitation import in a later release.

## 6. Decide what infrastructure is authoritative

KnotRoute deliberately splits responsibilities:

| Component | What it knows | What it does not control |
| --- | --- | --- |
| Beacon | short-lived signed node endpoint announcements for a network ID | `.knot` service catalog, payloads, naming |
| Bootstrap relay | adjacent peer connections/circuit hop state | complete onion path, service plaintext |
| Directory replicas | signed service descriptors assigned by XOR proximity | service private key, application payload |
| Introduction point | a service's reverse introduction circuit | service application plaintext |
| Rendezvous | two circuit ends to bridge | end-to-end service payload key |
| Network operator | bootstrap endpoints and operational policy | no mandatory central lookup/control plane |

Do not build operational procedures that assume a Beacon can list all services or users; the protocol intentionally does not provide that information.

## 7. Naming policy

Canonical service addresses are cryptographic service identities:

```text
<service-identity>.knot
```

Human-readable aliases such as:

```text
docs.knot
```

are local mappings. Current v3 has no global pretty-name registry.

If you publish a curated alias list outside KnotRoute, treat it as an external directory product/policy, not as an intrinsic protocol truth. Preserve the canonical service identity alongside every friendly name so users can verify what the alias points to.

## 8. Browser CA policy

There is **no network-wide shared Root CA** in v3.

Each client creates its own local KnotRoute Root CA and trusts only that CA on that device. The local proxy issues short-lived certificates for `.knot` names. The remote service is authenticated separately through its service identity and rendezvous cryptography.

This design avoids handing a central CA private key to network infrastructure.

Operational implications:

- never distribute a shared CA private key;
- users install only their own local KnotRoute public root;
- removing/recreating a client data directory changes that device's local CA;
- a compromised client CA affects `.knot` browser TLS on that device, not every network participant.

## 9. Service policy and access control

Published hidden services intentionally conceal the client's node identity from the service path. In current v3, anonymous published services therefore use:

```json
"allow": ["*"]
```

If a published service must be restricted, require application-layer authentication.

Direct node-address services can use node-ID ACLs, but they have different privacy properties and should not be confused with hidden-service rendezvous.

The network operator should publish this distinction in any deployment policy so service operators do not accidentally assume `network_id` or an unguessable `.knot` address is access control.

## 10. Operational monitoring

Monitor each public infrastructure node independently.

### Beacons

```text
GET https://beacon-a.example.net/healthz
```

Expected:

```json
{"ok":true}
```

### Relays

Use local dashboard status or a host-local agent against:

```text
GET http://127.0.0.1:8484/api/health
GET http://127.0.0.1:8484/api/status
```

Important fields include:

```text
network_id
peers
routes
active_circuits
descriptors
bytes_sent / bytes_received
events
```

### End-to-end synthetic monitoring

The strongest availability check is a real client in another failure domain opening a known test service through its canonical `.knot` identity.

Process health alone does not detect a partitioned overlay, failed descriptor propagation, or rendezvous failure.

## 11. Backup plan

Back up:

- Beacon relay `/data` volumes if stable relay identities are desired;
- baseline relay identities/configuration;
- any operator-owned service identity files;
- the canonical `network_id` and bootstrap configuration;
- generated invitation files if you need reproducible onboarding artifacts.

Service operators are individually responsible for their service private identities.

The network itself does not depend on one master private key in v3.

## 12. Upgrades

Use rolling upgrades for always-on infrastructure:

1. Upgrade one Beacon/relay.
2. Verify health and that it rejoins the correct `network_id`.
3. Perform an end-to-end `.knot` request through the network.
4. Continue to the next infrastructure node.
5. Publish client upgrades after baseline infrastructure is known-good.

For protocol-breaking future releases, run a compatibility plan rather than assuming mixed versions interoperate.

## 13. Incident response

### Beacon compromise

A Beacon does not possess service private keys. Replace/rebuild it, change its relay identity if necessary, and remove the compromised Beacon URL from onboarding/configuration.

Because clients verify peer identities at the KnotRoute handshake, a malicious Beacon can provide bad candidates or deny discovery, but it cannot make one key authenticate as another node.

### Relay compromise

Remove it from static seeds/bootstrap references and rebuild with a new identity. Existing nodes may retain the old endpoint in peer cache until expiry/replacement, so distribute updated configuration where necessary.

### Service key compromise

The service identity is compromised. Stop publication, create a new service identity, and distribute the new canonical `.knot` address through a trusted channel. Current v3 has no global service-key revocation registry.

### Network ID disclosure

This is not, by itself, a cryptographic compromise because the network ID was never an admission secret. If unauthorized participation must be cryptographically prevented, current v3 requires an additional membership/admission mechanism; changing only the network ID is a disruptive namespace migration, not a robust revocation system.

## 14. Recommended baseline deployment

A reasonable starting layout is:

```text
                 HTTPS Beacon A + relay A
                         │
       ┌─────────────────┼─────────────────┐
       │                 │                 │
   relay C           relay D          relay E
       │                 │                 │
       └─────────────────┼─────────────────┘
                         │
                 HTTPS Beacon B + relay B

        clients / service hosts join dynamically
```

Operational baseline:

- one generated non-default `network_id`;
- 2+ Beacon URLs;
- 5+ stable relay-capable nodes when practical;
- persistent identities/volumes;
- TLS in front of Beacon HTTP APIs;
- direct TCP reachability for advertised relay endpoints;
- PEX + peer cache enabled;
- external synthetic `.knot` health check;
- documented service-key backup policy;
- application-layer authentication for restricted published services.

This produces a self-hosted encrypted service overlay with decentralized service discovery/rendezvous while keeping bootstrap infrastructure replaceable rather than authoritative.
