# Docker deployment

KnotRoute includes two dedicated server images in addition to the ordinary node image.

## Sidecar: publish existing containers

`Dockerfile.sidecar` builds `knotroute-sidecar` as a static scratch-based runtime image.

The normal pattern is:

```text
Internet/host ports: none for the application

KnotRoute sidecar ─ Docker network ─ application:port
       │
       └─ overlay listener / outbound peers
```

Minimal compose example:

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

networks:
  private:

volumes:
  knotroute-data:
```

The application port is reachable only inside the Docker network unless some other stack intentionally publishes it.

A container may simultaneously be attached to a normal reverse-proxy network and to KnotRoute's sidecar network. KnotRoute does not require replacing Traefik, Caddy, nginx, or another public ingress.

### Multiple services

Use `KNOTROUTE_SERVICES_JSON`:

```json
[
  {
    "name": "web",
    "target": "frontend:8080",
    "publish": true,
    "intro_count": 3,
    "allow": ["*"]
  },
  {
    "name": "api",
    "target": "backend:9000",
    "publish": true,
    "intro_count": 3,
    "allow": ["*"]
  }
]
```

Persist `/data`. Service identity files live there; deleting them changes the corresponding canonical service addresses.

### Sidecar environment

```text
KNOTROUTE_DATA_DIR
KNOTROUTE_CONFIG
KNOTROUTE_NETWORK_ID
KNOTROUTE_LISTEN
KNOTROUTE_ADVERTISE
KNOTROUTE_BEACONS
KNOTROUTE_LAN
KNOTROUTE_SERVICE_NAME
KNOTROUTE_SERVICE_TARGET
KNOTROUTE_SERVICES_JSON
KNOTROUTE_HEALTH_LISTEN
```

Health endpoint:

```text
GET :9090/healthz
```

## Beacon/bootstrap relay

`Dockerfile.beacon` builds a small peer-discovery service. It keeps signed peer registrations only in memory. The optional relay identity is persisted under `/data`.

```bash
docker compose -f compose.beacon.yaml up -d --build
```

Configuration:

```text
KNOTROUTE_BEACON_LISTEN
KNOTROUTE_BEACON_TTL
KNOTROUTE_BEACON_MAX_NETWORK_PEERS
KNOTROUTE_BEACON_RATE
KNOTROUTE_BEACON_BURST
KNOTROUTE_BEACON_RELAY
KNOTROUTE_BEACON_RELAY_LISTEN
KNOTROUTE_BEACON_RELAY_ADVERTISE
KNOTROUTE_BEACON_DATA
KNOTROUTE_NETWORK_ID
```

The Beacon HTTP API stores no service descriptors and sees no application payload. If bundled relay mode is enabled, the same container also acts as an ordinary KnotRoute router on its relay port.

For resilient deployments configure clients with multiple independent Beacons.
