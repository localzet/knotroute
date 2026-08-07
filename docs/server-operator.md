# Server operator guide

This guide covers KnotRoute nodes that run continuously on servers: relays, service hosts, Docker sidecars, and bootstrap infrastructure.

## Decide which process you need

```text
knotroute          general-purpose node/CLI
knotroute-sidecar  container-oriented service publisher
knotroute-beacon   peer-discovery server + optional bootstrap relay
```

A server can run more than one role, but keep identities and listening ports separate unless the role intentionally shares one process.

## Network ports

Typical defaults:

```text
7447/tcp  KnotRoute peer/relay transport
8080/tcp  Beacon HTTP API, only when running Beacon
9090/tcp  sidecar health endpoint, if exposed/used
```

Dashboard and local proxies should normally remain loopback-only:

```text
8484/tcp  dashboard, default 127.0.0.1 only
9477/tcp  SOCKS5, default 127.0.0.1 only
9478/tcp  HTTP proxy, default 127.0.0.1 only
```

Do not publish the dashboard management API to an untrusted network. The code restricts management requests to loopback, but network exposure is unnecessary and provides no benefit for a server role.

## Listener versus advertised endpoint

`listen` is where the process binds locally:

```json
"listen": ["0.0.0.0:7447"]
```

`advertise` is what other peers should dial:

```json
"advertise": ["node1.example.net:7447"]
```

If the server is behind NAT or a load balancer, `advertise` must describe the externally reachable TCP endpoint, not the container's private address.

A client-only node with no inbound reachability may leave `advertise` empty. It can still use outbound peers and Beacon discovery.

## Run a generic node on Linux

Create configuration and identity:

```bash
./knotroute init --config /etc/knotroute/knotroute.json
```

Edit the configuration, then validate:

```bash
./knotroute doctor --config /etc/knotroute/knotroute.json --probe
```

Run interactively first:

```bash
./knotroute run --config /etc/knotroute/knotroute.json
```

The repository includes a systemd unit and install helper under `packaging/systemd/`. Review paths and ownership for your distribution before installing it.

## Docker sidecar

The sidecar generates its initial config from environment variables and persists identities under `/data`.

Minimal example:

```yaml
services:
  app:
    image: your/application:latest
    networks: [knot-private]

  knotroute:
    image: knotroute-sidecar:3.0.0
    restart: unless-stopped
    networks: [knot-private]
    volumes:
      - knotroute-data:/data
    environment:
      KNOTROUTE_NETWORK_ID: "kn_..."
      KNOTROUTE_BEACONS: "https://beacon-a.example,https://beacon-b.example"
      KNOTROUTE_SERVICE_NAME: "web"
      KNOTROUTE_SERVICE_TARGET: "app:8080"
      KNOTROUTE_LISTEN: "0.0.0.0:7447"
      KNOTROUTE_ADVERTISE: "node.example.net:7447"

networks:
  knot-private:

volumes:
  knotroute-data:
```

If inbound peering is desired, publish `7447/tcp` and set an externally reachable `KNOTROUTE_ADVERTISE`. If the sidecar is outbound-only, no host port is required and `KNOTROUTE_ADVERTISE` may remain empty.

The sidecar health endpoint defaults to:

```text
http://container:9090/healthz
```

It reports process/node health only.

## Coexist with Traefik

A common layout is:

```text
                       public HTTPS
                           │
                     Traefik/Caddy
                           │
                           ▼
                       application
                           ▲
                           │ private Docker network
                           │
                    KnotRoute sidecar
                           │
                       .knot overlay
```

The application may participate in both the public reverse-proxy network and a private KnotRoute network. KnotRoute targets the container's internal hostname/port and does not require a second public port.

For a hidden web site, target the application's HTTP listener. Browser-facing `.knot` TLS is terminated locally on each client and the KnotRoute rendezvous session already provides independent authenticated end-to-end encryption.

## Run a Beacon/bootstrap relay

Build/start:

```bash
docker compose -f compose.beacon.yaml up -d --build
```

Important environment variables:

```text
KNOTROUTE_NETWORK_ID
KNOTROUTE_BEACON_LISTEN
KNOTROUTE_BEACON_TTL
KNOTROUTE_BEACON_MAX_NETWORK_PEERS
KNOTROUTE_BEACON_RATE
KNOTROUTE_BEACON_BURST
KNOTROUTE_BEACON_RELAY
KNOTROUTE_BEACON_RELAY_LISTEN
KNOTROUTE_BEACON_RELAY_ADVERTISE
KNOTROUTE_BEACON_DATA
```

Health:

```bash
curl -fsS http://127.0.0.1:8080/healthz
```

Expected body:

```json
{"ok":true}
```

For production, expose Beacon discovery through HTTPS, usually using an existing reverse proxy. The relay port is raw KnotRoute TCP and should be exposed as TCP, not as an HTTP route.

## Persistent data

Treat these as state, not disposable runtime files:

- node identity;
- service identities;
- Beacon bundled-relay identity;
- peer cache;
- configuration.

For sidecar/Beacon containers, persist `/data`.

The most critical backup is each **service identity**. Losing it changes the canonical `.knot` address.

## Upgrade procedure

Recommended sequence:

1. Back up the persistent data directory/volume.
2. Read `CHANGELOG.md` and release notes.
3. Run the new binary against a copy of the configuration with `doctor` if configuration semantics changed.
4. Upgrade one relay/Beacon at a time when operating redundant infrastructure.
5. Confirm `/healthz`, connected peers, routes, and an external `.knot` request.
6. Only then roll the remaining nodes.

Do not replace identity files during a routine upgrade.

## Monitoring

Available checks include:

- Beacon: `GET /healthz`;
- sidecar: `GET /healthz` on the configured health listener;
- node dashboard: `GET /api/health` and `GET /api/status` on the loopback dashboard listener;
- external synthetic check: resolve/open a known `.knot` service from a separate client.

For meaningful availability monitoring, use both process-level and end-to-end checks. A running process can still be isolated from the overlay.

## Capacity and topology

There is no universal relay count. For the current defaults:

- circuits request three hops where topology allows;
- descriptors request five replicas where enough directory nodes are known.

A tiny two-node network can function for direct cases, but it cannot provide the path diversity implied by the default privacy settings. For a continuously available independent deployment, operate multiple always-on relays across independent hosts/networks and use more than one Beacon.

## Hardening checklist

- keep KnotRoute and application containers unprivileged;
- never mount Docker socket into KnotRoute containers;
- expose only required ports;
- use HTTPS for Beacon HTTP endpoints;
- keep dashboards/local proxies on loopback unless there is a deliberate trusted-network design;
- protect `/data` backups as secrets;
- use application-layer authentication for restricted hidden services;
- monitor disk, memory, file-descriptor, and connection counts at the host level;
- rate-limit and firewall public infrastructure appropriately;
- read `SECURITY.md` and do not describe v3 as Tor-equivalent anonymity.
