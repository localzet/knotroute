# Embedding KnotRoute

KnotRoute exposes application-facing packages so software can participate in the overlay without requiring a separately installed desktop client.

## Go client: `pkg/knotclient`

```go
client, err := knotclient.New(knotclient.Options{
    DataDir:       "./state",
    NetworkID:     "kn_...",
    Beacons:       []string{"https://beacon.example"},
    CircuitHops:   3,
    EnableLAN:     true,
    PeerExchange:  true,
    EnableLocalCA: false,
})
if err != nil {
    return err
}
if err := client.Start(ctx); err != nil {
    return err
}
defer client.Close()

conn, err := client.Dial(ctx, "<service>.knot")
```

`Dial` returns a `net.Conn`, so existing stream-oriented protocols can usually be reused unchanged.

`HTTPClient()` supplies a normal `net/http.Client` whose dialer understands plain-HTTP `.knot` destinations while leaving ordinary Internet hosts on the normal network stack.

## Go server: `pkg/knotserver`

The server package publishes handlers without requiring the application to open a public TCP listener.

```go
host, err := knotserver.New(knotserver.Options{
    DataDir: "./state",
    Beacons: []string{"https://beacon.example"},
    Services: []knotserver.Service{{
        Name: "echo",
        Handler: knotserver.HandlerFunc(func(conn net.Conn) {
            _, _ = io.Copy(conn, conn)
        }),
    }},
})
if err != nil {
    return err
}
if err := host.Start(ctx); err != nil {
    return err
}
defer host.Close()

address, _ := host.ServiceAddress("echo")
fmt.Println(address)
```

The service address remains stable while its service identity file in `DataDir/services/` is preserved.

## Android AAR

`mobile/knotmobile` is intentionally a narrow gomobile-safe API. The generated AAR supports:

- persistent embedded node identity;
- network ID and Beacon configuration;
- automatic discovery;
- local HTTP proxy;
- local Root CA access;
- local TCP forwards to `.knot` services.

An application can therefore bundle the AAR and provide KnotRoute connectivity without asking users to install the standalone KnotRoute application.

## Application primitives

### RPC

`knotapp.RPCClient` and `RPCServer` provide bounded framed JSON request/response calls with explicit method names and structured error responses.

### Datagram

`DatagramClient`/`DatagramServer` map one bounded message onto one short-lived KnotRoute stream. This provides message semantics without claiming UDP transport semantics.

### Pub/Sub

`PubSubClient` and `PubSubBroker` support brokered topics where the topic secret encrypts payloads before they reach the broker. The broker routes opaque topic messages.

### Objects

`ObjectClient`/`ObjectServer` use SHA-256 content IDs and verify object integrity on read. `FileObjectStore` provides a local persistent implementation.

### Offline mailbox

`pkg/knotmailbox` provides encrypted envelopes for store-and-forward delivery. Mailbox storage is designed to hold ciphertext rather than message plaintext; recipients authenticate fetch/ack operations and explicitly acknowledge deletion.

These primitives are optional. The core `net.Conn` API remains the lowest common denominator for arbitrary application protocols.
