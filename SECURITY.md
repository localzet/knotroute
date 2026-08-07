# Security policy

## Reporting a vulnerability

Please do not open a public issue for a vulnerability that exposes identities, plaintext, private keys, authentication bypasses, or remotely exploitable crashes.

Send a private GitHub security advisory to the repository owner with:

- affected version and platform;
- minimal reproduction;
- expected and observed behavior;
- impact assessment;
- suggested fix, when available.

You should receive acknowledgement within seven days. Do not include real private keys or production service credentials.

## Supported version

The repository currently supports version 1.0.x. Security fixes should be released as a new patch tag with checksums and a concise advisory.

## Deployment guidance

- restrict `identity.json` to the node account;
- pin seed `expected_id` values where practical;
- keep the dashboard on loopback;
- use explicit service ACLs;
- retain application-layer authentication;
- avoid advertising private addresses that should not be disclosed to overlay peers.
