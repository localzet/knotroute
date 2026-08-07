# Publishing KnotRoute

KnotRoute publishes two kinds of artifacts:

1. GitHub Release artifacts: desktop/server archives, Android APKs, and the Android AAR SDK.
2. OCI container images in GitHub Container Registry (GHCR).

## Release versioning

Do not move an already published tag. For a build/CI fix after `v3.0.1`, publish `v3.0.2`.

```bash
git tag v3.0.2
git push origin v3.0.2
```

The `Release` workflow creates platform archives plus Android artifacts. The `Containers` workflow publishes three multi-architecture images:

```text
ghcr.io/localzet/knotroute:3.0.2
ghcr.io/localzet/knotroute-sidecar:3.0.2
ghcr.io/localzet/knotroute-beacon:3.0.2
```

It also publishes `3.0` and `latest` for a version tag. Pushes to `main` publish the `edge` tag.

## Android signing

The release workflow can sign the release APK when these repository secrets are configured:

```text
ANDROID_KEYSTORE_BASE64
ANDROID_KEYSTORE_PASSWORD
ANDROID_KEY_ALIAS
ANDROID_KEY_PASSWORD
```

Create and keep the keystore outside the repository. Encode it for the secret with:

```bash
base64 -w0 knotroute-release.jks
```

If the secrets are absent, the workflow still publishes the debug APK, unsigned release APK, and AAR SDK.

## GHCR visibility

The workflow authenticates with the repository `GITHUB_TOKEN`; no PAT is required. After the first publish, verify the package visibility in GitHub Packages. If the images should be publicly pullable, set each package to Public and link it to the repository if GitHub did not do so automatically.

## Production Beacon

The Beacon HTTP API is normally placed behind HTTPS while the relay stays raw TCP:

```yaml
services:
  beacon:
    image: ghcr.io/localzet/knotroute-beacon:3.0.2
    restart: unless-stopped
    ports:
      - "7447:7447"
    expose:
      - "8080"
    volumes:
      - knotroute-beacon:/data
    environment:
      KNOTROUTE_NETWORK_ID: "kn_..."
      KNOTROUTE_BEACON_LISTEN: "0.0.0.0:8080"
      KNOTROUTE_BEACON_RELAY: "true"
      KNOTROUTE_BEACON_RELAY_LISTEN: "0.0.0.0:7447"
      KNOTROUTE_BEACON_RELAY_ADVERTISE: "relay.example.net:7447"

volumes:
  knotroute-beacon:
```

Route `https://beacon.example.net` to container port `8080` using Traefik/Caddy/nginx. Do not send TCP `7447` through an HTTP router.

## Publish an existing application as a `.knot` service

The application needs no public host port for KnotRoute:

```yaml
services:
  app:
    image: example/app:latest
    networks: [knot]

  knotroute:
    image: ghcr.io/localzet/knotroute-sidecar:3.0.2
    restart: unless-stopped
    networks: [knot]
    volumes:
      - knotroute-data:/data
    environment:
      KNOTROUTE_NETWORK_ID: "kn_..."
      KNOTROUTE_BEACONS: "https://beacon-a.example.net,https://beacon-b.example.net"
      KNOTROUTE_SERVICE_NAME: "web"
      KNOTROUTE_SERVICE_TARGET: "app:8080"

networks:
  knot:

volumes:
  knotroute-data:
```

The sidecar prints the generated canonical service address at startup. Retrieve it with `docker compose logs knotroute` and preserve the `/data` volume: deleting its service identity changes the `.knot` address.

An application may simultaneously remain attached to a Traefik network for public Internet ingress and to a private KnotRoute network for `.knot` publication.
