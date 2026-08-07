# Security policy

## Reporting a vulnerability

Do not open a public issue for a vulnerability that exposes identities, plaintext, private keys, authorization bypasses, route-control vulnerabilities, CA private keys, or remotely exploitable crashes.

Use a private GitHub security advisory for the repository and include:

- affected version and platform;
- minimal reproduction;
- expected and observed behavior;
- impact assessment;
- suggested fix, when available.

Do not include real production private keys, invitations, or application credentials.

## Supported version

The current supported major release is 3.x. Security fixes should be released as a new patch tag with updated checksums and a concise advisory.

## Security properties

KnotRoute v3 implements authenticated peer identities, TLS 1.3 direct links, signed topology records, telescopic onion-style circuits, independent service identities, signed descriptors, introduction/rendezvous paths, and an additional end-to-end encrypted session between a hidden-service client and service.

These mechanisms are implemented security controls. They are **not** a claim of Tor-equivalent anonymity or evidence of independent protocol audit.

## Threat-model limits

Do not assume KnotRoute defeats:

- a global passive observer;
- timing or volume correlation;
- a large Sybil population;
- malicious-majority topology control;
- application/browser fingerprinting;
- endpoint compromise;
- denial of service;
- identifying information intentionally or accidentally transmitted by an application.

The current link-state layer intentionally reveals router identities and topology to participating routers. Directory replicas see service IDs whose descriptors they store or answer. Introduction/rendezvous routers observe their local circuit relationships and timing.

## Local Root CA

The optional browser integration creates a per-device KnotRoute Root CA. Its private key is stored locally and the issuer refuses non-`.knot` names, but installing any Root CA increases local trust surface.

Recommended handling:

- install it only on devices where `.knot` browser UX is needed;
- protect the KnotRoute data directory with the operating-system account boundary;
- remove the CA when KnotRoute system integration is no longer used;
- rotate/delete the CA if its private key might have been copied;
- do not distribute one device's CA private key to other devices.

The CA is not used to authenticate overlay services. Hidden-service identity is independently verified with the service Ed25519 key.

## Deployment guidance

- protect node and service identity files and back them up only when address continuity is required;
- keep the management dashboard on loopback unless it is placed behind a separate authenticated local management layer;
- configure multiple independent bootstrap sources for availability;
- use application-layer authentication and authorization for sensitive services;
- treat human-readable aliases as local trust decisions;
- explicitly advertise only transport endpoints that may be disclosed to participating peers;
- keep Docker application ports unexposed when KnotRoute is intended to be the only ingress;
- update clients together when protocol compatibility changes.
