# Shardwave

Open-source, decentralized P2P system for running AI models too large for a
single GPU across multiple independent machines, via distributed model
inference.

**Status:** pre-Sprint 001. Documentation only; no code yet.

**This is a learning project.** It is not a product and does not intend to
become one. Decisions optimize for understanding distributed systems and
distributed inference, not for shipping. Read
[`docs/NORTH_STAR.md`](docs/NORTH_STAR.md) first.

## Shape

- **Go** — the node: identity, discovery, transport, peer table, orchestration.
- **Python + PyTorch** — the execution worker, behind a boundary the Go node
  controls.
- Two machines, two architectures, one LAN: an Apple M4 Mac and a Windows PC
  with an RTX 5070.

## Documents

- [North Star](docs/NORTH_STAR.md) — what we are building, why, the
  architecture, and the roadmap.
- [Architecture Decision Records](docs/adr/) — contested decisions and when to
  revisit them.
- [Experiments](docs/experiments/) — measured results, from Sprint 004 on.

## Roadmap at a glance

| Sprint | Outcome |
|---|---|
| 001 | Two Go nodes: authenticated handshake, liveness, graceful shutdown, across the LAN |
| 002 | The Go ↔ Python boundary: a matmul computed on one node continues on the other |
| 003 | GPT-2 split across two machines, layer-wise |
| 004 | KV cache, benchmarks, and what breaks when you pull the cable |
| 005 | Graduate to a modern small model |

## License

[MIT](LICENSE)
