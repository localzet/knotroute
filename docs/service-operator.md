# Service operator guide

This guide is for the owner of an HTTP site, API, SSH endpoint, database, game server, message service, or any other TCP application that should be reachable through KnotRoute.

A published v3 service has an independent cryptographic identity. Its canonical `.knot` address does not contain the hosting node identity, so the service can move between hosts without changing its address as long as the service identity file is preserved.

## Choose direct or published service mode

KnotRoute supports two models.

### Published service

Recommended for normal v3 use:

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

The service receives its own canonical address:

```text
<service-identity>.knot
```

The client discovers signed introduction points and reaches the service through separate client/service circuits meeting at a rendezvous node.

### Direct service

A direct service is addressed through a node identity, for example:

```text
ssh.<node-address>.knot
```

This is useful for administration, diagnostics, and node-specific endpoints, but it exposes the destination node identity to the route and is not the preferred hidden-service mode.

## Publish a web site on a native KnotRoute node

Start with a local application listening on a private/local address such as:

```text
127.0.0.1:8080
```

Add a published service to `knotroute.json`:

```json
{
  "services": [
    {
      "name": "web",
      "target": "127.0.0.1:8080",
      "description": "Web application",
      "publish": true,
      "identity_file": "services/web.identity.json",
      "intro_count": 3,
      "allow": ["*"],
      "metadata": {
        "protocol": "http"
      }
    }
  ]
}
```

Restart KnotRoute, then print the stable service address:

```bash
knotroute address --config knotroute.json --service web
```

Back up `services/web.identity.json`. The `.knot` address is derived from that identity.

## HTTP and HTTPS behavior

For a canonical published service opened as:

```text
https://<service>.knot/
```

browser TLS is terminated by the **client's local KnotRoute proxy** using that client's local KnotRoute Root CA. The resulting HTTP byte stream is then carried through the separately authenticated/encrypted KnotRoute service session.

Therefore a typical hidden web service should expose ordinary HTTP to its local KnotRoute target:

```text
KnotRoute sidecar/node -> app:8080 (HTTP)
```

It does not need a public WebPKI certificate and it does not need to listen on host port 443.

If your application generates absolute URLs, canonical-host redirects, OAuth callback URLs, CSP origins, cookie domains, or WebSocket URLs, configure the application to understand its `.knot` external origin. Otherwise the application itself may redirect users to an ordinary public hostname.

## Publish an existing Docker application

The sidecar pattern is usually the easiest option:

```yaml
services:
  app:
    image: nginx:alpine
    restart: unless-stopped
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
      KNOTROUTE_NETWORK_ID: "kn_..."
      KNOTROUTE_BEACONS: "https://beacon-a.example,https://beacon-b.example"
      KNOTROUTE_SERVICE_NAME: "web"
      KNOTROUTE_SERVICE_TARGET: "app:80"

networks:
  private:

volumes:
  knotroute-data:
```

The application itself has no host `ports:` mapping in this example.

If the same application is also published publicly through Traefik/Caddy/nginx, attach it to both networks. KnotRoute does not need to replace the existing public ingress.

## Multiple services from one sidecar

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

Each published service gets its own independent identity and `.knot` address.

## Service access control

There is an important v3 distinction:

- direct services can authorize specific source node IDs through `allow`;
- a hidden-service rendezvous circuit intentionally does not expose the client's node identity to the service path, so current v3 anonymous published services require `allow: ["*"]`.

For restricted published services, enforce authorization at the application layer: user accounts, application keys, mutually authenticated application protocol, signed capability tokens, or another scheme appropriate to the application.

Do not treat an unlisted `.knot` address or an obscure `network_id` as authorization.

## Stable identity and migration

The stable address is controlled by the service private identity file, not by the server.

To migrate a service:

1. Stop the old publisher.
2. Copy the service identity file securely to the new host.
3. Configure the same service name/identity path and new local target.
4. Start the new publisher.
5. Wait for the newer signed descriptor to propagate.
6. Verify the canonical `.knot` address did not change.

Avoid running two unrelated instances with copies of the same service private key unless you intentionally understand and control descriptor revision behavior. Treat the service key as a production secret.

## Backup policy

Back up at minimum:

```text
service identity file(s)
node identity if node continuity matters
knotroute.json
```

For Docker sidecars, persist and back up `/data`; service identities are created under that volume.

Loss of a service private identity means loss of the canonical service address. Restoring application data without restoring the service identity will create a new `.knot` identity.

## Availability recommendations

For an important service:

- configure multiple Beacon URLs;
- use several introduction points (`intro_count: 3` is the normal baseline);
- keep the hosting node connected to more than one relay when possible;
- do not expose the dashboard publicly;
- monitor the sidecar/node process and local target independently;
- preserve service identity backups before upgrades or host migration.

A successful local health check proves the KnotRoute process is alive; it does not prove that remote directory lookup and rendezvous are currently possible. Perform an external `.knot` request from another KnotRoute client for end-to-end monitoring.

## Application security still matters

KnotRoute protects transport and service addressing. It does not fix vulnerabilities in the published application. Continue to apply normal controls for authentication, authorization, CSRF, XSS, SQL injection, dependency updates, session security, and secrets management.
