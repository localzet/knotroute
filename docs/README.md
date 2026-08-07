# KnotRoute documentation

This directory is organized by operational role. Start with the guide that matches what you are trying to do.

| Role | Guide | Purpose |
| --- | --- | --- |
| End user | [User guide](user-guide.md) | Install a client, join a network, trust the local CA, and open `.knot` services. |
| Service operator | [Service operator guide](service-operator.md) | Publish an HTTP site or TCP service and keep its `.knot` identity stable. |
| Server operator | [Server operator guide](server-operator.md) | Run relays, sidecars, Docker workloads, system services, backups, and upgrades. |
| Network operator | [Network operator guide](network-operator.md) | Create and operate an independent KnotRoute network, Beacons, bootstrap relays, invitations, and baseline infrastructure. |
| Application developer | [SDK guide](sdk.md) | Embed KnotRoute into Go or Android applications and use application primitives. |
| Windows administrator/user | [Windows guide](windows.md) | Desktop tray, system proxy integration, local CA, and Windows Service mode. |
| Android user/developer | [Android guide](android.md) | Build and operate the standalone Android client or embed the AAR. |
| Protocol implementer | [Protocol](protocol.md) | Wire protocol, circuits, directory, and rendezvous internals. |
| Container operator | [Docker deployment](docker.md) | Sidecar and Beacon container details. |

Before using KnotRoute for sensitive workloads, read [`SECURITY.md`](../SECURITY.md).

## Terminology

- **Node**: a KnotRoute router identified by a node identity.
- **Service**: an application endpoint published by a node.
- **Published service / hidden service**: a service with its own independent service identity and canonical `.knot` address.
- **Direct service**: a service addressed through a node address, such as `http.<node>.knot`.
- **Beacon**: a soft-state peer-discovery endpoint. It is not a service directory or naming authority.
- **Bootstrap relay**: an always-reachable KnotRoute node that provides a first overlay edge to new clients.
- **Network ID**: separates independent KnotRoute overlays. It is not a password or membership credential.
- **Local CA**: a per-device certificate authority used only to make browser HTTPS work for `.knot`; it is not shared across the network.
