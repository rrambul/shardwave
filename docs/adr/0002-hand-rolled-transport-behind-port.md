# ADR-0002 — Hand-rolled transport behind a port

- **Status:** Accepted
- **Date:** 2026-11-08

## Context

The obvious choice for P2P networking in Go is `go-libp2p`: the reference
implementation, with peer IDs, multiaddrs, stream multiplexing, Noise
encryption, mDNS, Kademlia DHT, NAT traversal, and circuit relays.

Against that:

- Learning goal #1 is distributed systems in Go. Adapting libp2p's echo
  example teaches libp2p's abstractions, not what they abstract over.
- Horizon C (trusted LAN mesh) needs none of libp2p's differentiating
  features. It needs TCP, framing, identity, liveness, and eventually mDNS.
- We want to remain implementation-agnostic so that libp2p (Go, or in
  principle another implementation) can be adopted later without rewriting the
  core.

## Decision

### The port

```go
type PeerID [32]byte // sha256(ed25519 pubkey). Never an ip:port. Never a libp2p peer.ID.

type Transport interface {
    Listen(ctx context.Context, addr string) error
    Connect(ctx context.Context, addr string) (PeerID, error)
    Send(ctx context.Context, to PeerID, payload []byte) error
    Recv(ctx context.Context) (from PeerID, payload []byte, err error)
    Events() <-chan PeerEvent // Connected | Disconnected
    Close() error
}

type Discovery interface {
    Peers(ctx context.Context) ([]string, error) // addrs for Transport.Connect
}
```

The port is **message-oriented** and addressed by **logical identity**. It
does not expose connections or streams.

### The first adapter

`transport/tcp`: hand-rolled on `net`, `crypto/ed25519`, `crypto/sha256`.
Length-prefixed framing, a handshake that exchanges public keys and signs a
nonce, ping/pong liveness, goroutine-per-connection with `context` for
lifecycle.

### What makes us implementation-agnostic

Not the interface — the **`protocol` package**. Framing, message types,
versioning, and error semantics live in the core and know nothing about
transports. Adapters move `[]byte`. A future libp2p adapter wraps our frames
in a libp2p stream; the core does not change.

### Guardrails

1. No streams in the port. Multiplexing is adapter-internal when needed.
2. One adapter per port until a trigger below fires. Zero speculative
   adapters.
3. No contract-test suite for adapters that do not exist. Tests target the
   port via `transport/tcp`; adapter #2 runs the same tests when it arrives.
4. Discovery is a separate port from transport. Adapter #1 is a static peer
   list; adapter #2 is mDNS.

## Consequences

- We build the bottom ~10% of libp2p ourselves and learn why the other 90%
  exists.
- We do not get NAT traversal, relays, DHT, Noise encryption, or stream
  multiplexing. Horizon C does not need them.
- Hand-rolled TCP is not throwaway: it *is* the Horizon C transport.
- Wire-level interoperability between libp2p implementations is free when we
  adopt one; it is not something we build.

## Revisit when

Any one of these is sufficient to open an ADR evaluating `go-libp2p` as
adapter #2:

- We need peers across NATs or over the Internet (Horizon B).
- We need discovery beyond a static list and mDNS.
- We find ourselves implementing stream multiplexing or transport encryption.
- The mesh exceeds roughly 20 peers and connection management becomes the
  work.
- A concrete second adapter is actually required for a concrete reason.

"We could" is not a trigger. "We are now reimplementing X" is.
