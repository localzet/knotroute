# Examples

`basic-node.json` is a valid standalone service-node configuration after an identity has been created in the same directory:

```bash
knotroute init --config generated.json
cp basic-node.json generated.json
knotroute doctor --config generated.json --probe
```

Peer and forward node IDs are intentionally not hard-coded in repository examples: they must be copied from the identities generated for your own nodes.
