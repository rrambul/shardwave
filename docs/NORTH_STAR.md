# Shardwave — North Star

> Build an open-source, decentralized P2P system that lets AI models too large
> for a single GPU execute collaboratively across multiple independent machines
> through distributed model inference.

This document is the source of truth for *what we are building and why*. It
changes rarely and deliberately. Decisions that were contested or that we
expect to revisit live in [`docs/adr/`](adr/). Measured results live in
[`docs/experiments/`](experiments/).

---

## 1. What Shardwave is

Shardwave pools heterogeneous computing resources across independently
operated devices to run AI inference collaboratively, without a centralized
compute provider.

Concretely: a model is partitioned layer-wise into **stages**; each stage runs
on a different machine; activations flow from one stage to the next over the
network until a token comes out the other end.

## 2. Why this exists

**Learning is the priority.** Shardwave is not a product, a company, or a path
to profit. Every decision in this document optimizes for *understanding the
hard parts*, not for shipping. When a choice trades learning for speed, we
choose learning.

Learning goals, ranked:

1. **Distributed and P2P systems in Go** — peer identity, discovery,
   connection lifecycle, framing, failure detection. Go is both the tool and a
   learning target.
2. **ML systems: how distributed inference actually works** — pipeline
   parallelism, activation transfer, KV-cache placement, what breaks over a
   real network.
3. **Fault-tolerant coordination** — supervision, partial failure, recovery.
   Learned *by need*, in Go, at the moment the system first breaks.

## 3. Horizons

### Horizon C — Trusted Mesh (now)

Devices we control, on a LAN we control. Validates networking, distributed
execution, and model partitioning without adversarial concerns. We build it
without assuming future peers will be trustworthy, but we do not build trust
machinery yet.

### Horizon B — Decentralized Inference (north star)

Independently operated, geographically distributed nodes contributing GPU
resources over the Internet. Everything in Horizon C should be *layerable*
toward this — identity, protocol versioning, and the transport boundary are
designed with it in mind.

### Not in the initial scope

GPU marketplaces, payments, tokenomics, trustless verification of inference,
and production-grade orchestration. These are real problems and may well
become Shardwave's problems at Horizon B — but none is attempted until the
pipeline works and is measured. Horizon C's design should not foreclose them;
Horizon C's effort should not be spent on them.

See ADR-0001 for why we do not pursue OTP-based coordination.

## 4. Priorities

- **Primary — Model parallelism.** Partition one model across machines.
  Layer-wise pipeline execution first.
- **Secondary — Request parallelism.** Distribute independent requests for
  throughput. Later; only after the pipeline works and is measured.

## 5. Architecture

The minimum architecture that gets from "two nodes talk" to "cross-machine
model-parallel inference" without overengineering the first step.

### Go is the brain

The Go node owns peer identity, discovery, connections, framing, liveness, the
peer table, and — eventually — pipeline orchestration (which stage runs where,
routing activations between stages). Python is an execution worker behind a
boundary the Go node controls.

### Two ports, one protocol

```
                 ┌─────────────────────────── Go node ───────────────────────────┐
                 │                                                               │
  Discovery ────▶│  discovery.Discovery ──▶ node (peer table, lifecycle, relay)  │
   (port)        │                               │                 ▲             │
                 │                               ▼                 │             │
                 │                      transport.Transport    executor.Executor │
                 │                           (port)                (port)        │
                 └───────────────────────────────┼─────────────────┼─────────────┘
                                                 │                 │
                                            transport/tcp      executor/python
                                            (adapter #1)       (adapter #1)
                                                 │                 │
                                             the network      Python worker
                                                               (PyTorch)
```

- **`protocol`** — framing (`[u32 len][u8 type][payload]`), message types,
  versioning, error semantics. Lives in the core. Knows nothing about
  transports. *This package, not the interface, is what keeps us
  implementation-agnostic.*
- **`transport.Transport`** (port) — message-oriented, addressed by logical
  `PeerID`, never by `ip:port`. Exposes connect/send/recv/events/close. Does
  **not** expose streams. Adapter #1: hand-rolled TCP (ADR-0002).
- **`discovery.Discovery`** (port) — returns addresses to hand to
  `Transport.Connect`. Adapter #1: static list from flags. Adapter #2: mDNS.
- **`executor.Executor`** (port) — `Execute(stage, input Tensor) (Tensor,
  error)`. Adapter #0: a Go mock (matmul) so the node is testable with no
  Python. Adapter #1: a Python subprocess over loopback TCP (ADR-0003).

### Activation path

```
Executor A ──▶ node A ──▶ Transport ──▶ node B ──▶ Executor B
```

Go relays activations. The control/data-plane split that real systems
converge on is deliberately deferred until Sprint 004 measures the cost of not
having it (ADR-0003).

### Ports-and-adapters guardrails

This pattern inflates if unattended. Rules:

1. Message types, framing, and versioning live in `protocol`, not in adapters.
   Adapters move `[]byte`.
2. No streams in the transport port. Multiplexing is an adapter-internal
   concern when it arrives, or an ADR-0002 trigger.
3. No contract-test suite for adapters that do not exist. Tests target the
   port using adapter #1; when adapter #2 appears, the same tests run against
   it.
4. One adapter per port until a trigger fires. Zero speculative adapters.

## 6. Stack

| Layer | Choice | Notes |
|---|---|---|
| Core, networking, orchestration | **Go** | Standard library first. No libp2p until an ADR-0002 trigger fires. |
| Model execution | **Python + PyTorch** | Subordinate to the Go node. No networking beyond loopback. |
| Coordination runtime | **None beyond Go** | Elixir/OTP considered and rejected — ADR-0001. |
| Dashboard / observability | TypeScript, *future* | Not before Sprint 004 produces something worth looking at. |

Tooling: `uv` for Python, a `Makefile` for the Mac dev loop, `GOOS=windows`
cross-compilation for the PC.

## 7. Test bed

Two real machines, two architectures, one LAN. Horizon C is not simulated.

| Machine | Chip / GPU | Memory | Target | PyTorch backend |
|---|---|---|---|---|
| Mac | Apple M4 | 24 GB unified | `darwin/arm64` | MPS (or CPU) |
| Windows PC | RTX 5070 (Blackwell, `sm_120`) | 12 GB VRAM | `windows/amd64`, **native, not WSL2** | CUDA 12.8+, PyTorch ≥ 2.7 |

WSL2 is excluded because its virtual NAT breaks inbound LAN connections and
mDNS — the exact things under test. Native Windows is also *more*
heterogeneous, which is a feature.

## 8. Roadmap

Sprints are scoped by what they teach, not by calendar. Each ends with
something that runs and, from Sprint 004 on, something that is measured.

### Sprint 001 — P2P Foundation

Two Go nodes establish an authenticated connection and keep it alive.

**Done means:**

1. `shardwave node --listen :7000 --peers 127.0.0.1:7001` — two processes on
   the Mac.
2. **Identity** — each node generates or loads an ed25519 keypair;
   `PeerID = sha256(pubkey)`.
3. **Handshake** — exchange pubkeys, sign a nonce to prove ownership. Each side
   logs the other's PeerID. Not encryption; a trusted LAN does not need it yet.
4. **Framing** — `[u32 len][u8 type][payload]` in `protocol`. Types: `Hello`,
   `HelloAck`, `Ping`, `Pong`.
5. **Liveness** — ping every 2 s; three missed → `Disconnected` on
   `Transport.Events()`. Kill a node; the other notices within ~6 s.
6. **Peer table** — connected peers by PeerID with connect/disconnect
   transitions.
7. **Graceful shutdown** — Ctrl-C → `context` cancellation → every goroutine
   exits, nothing leaks. *This is the Go lesson of the sprint.*
8. **Tests** against the `Transport` port using `transport/tcp` on random
   ports: connect, send/recv round-trip, disconnect event fires.
9. **Cross-machine** — Go node on the Mac, `GOOS=windows` build on the PC,
   same handshake over the LAN. De-risks the Windows side before Sprint 003
   depends on it.

**Non-goals:** encryption, multiplexing, mDNS, reconnection/backoff, libp2p,
anything Python.

### Sprint 002 — The Boundary

The `Executor` port is born. Node A's Python worker computes `y = Wx`; node A
ships `y` to node B; node B's worker computes the next matmul. Also: the Go
mock executor, so Sprints 001–002 are testable with no Python installed.

Tensor payload format: small header (dtype, shape, stage id) + raw bytes,
safetensors-style. Never pickle.

### Sprint 003 — Model Parallelism POC

Replace the matmuls with real layers. **GPT-2 124M via nanoGPT** — ~300 lines
of pure PyTorch, real weights, `transformer.h[0:6]` on one node and `h[6:12]`
on the other. Runs on CPU in seconds.

Deliberately **without a KV cache**: per-token activations are
`[1, seq_len, 768]` and grow with every generated token. You watch the transfer
grow. The fix is the first task of Sprint 004.

Cross-machine (Mac ↔ PC) from the start.

### Sprint 004 — Measure

1. Add a per-stage KV cache. Activations shrink to `[1, 1, 768]`. Measure the
   before/after. This is why KV-cache *placement* is on the hard-problems list.
2. Benchmark single-machine vs two-stage pipeline: correctness (identical
   logits), tokens/s, latency per token, bytes per token, relay overhead as a
   fraction of per-token latency.
3. Pull the network cable mid-generation. Document exactly what state is lost
   and what would be needed to resume. This is where Sprint 005+ fault
   tolerance gets its requirements.

Relay-overhead number feeds the ADR-0003 revisit.

### Sprint 005 — Graduate the model

Move from GPT-2 to a modern small model (SmolLM2 / Qwen2.5-0.5B / Llama-3.2-1B
class — RoPE, GQA). The Python executor adapter grows up here, whether that
means HF layer modules or llama.cpp.

### Future (unordered, pulled in by need)

mDNS discovery · reconnection and recovery · heterogeneous scheduling (MPS vs
CUDA, uneven stage sizes) · request parallelism · GPU deployment on both
machines · libp2p adapter when an ADR-0002 trigger fires · untrusted WAN
participation (Horizon B) · dashboard.

## 9. Engineering principles

- Build incrementally and validate assumptions experimentally.
- Prefer minimal implementations over premature infrastructure.
- Keep networking, execution, discovery, and scheduling conceptually separate —
  the directory layout *is* the architecture diagram.
- Start in a trusted environment without assuming future peers are trustworthy.
- Treat WAN latency, activation transfer, KV-cache placement, and partial
  failure as the research, not as implementation details.
- Lock-in is cheap here; abstraction-for-swappability is paid daily. Write
  zero speculative adapters.
- Maintain this document, ADRs, and measured experimental results in the
  repository. An ADR has a **Revisit when** section. An experiment has numbers
  or it is not an experiment.

## 10. Hard problems we are walking toward

Named honestly so nobody is surprised.

| Problem | Why it is hard | When it bites |
|---|---|---|
| Per-token round trips | Pipeline inference needs one network traversal per stage per token. Over WAN this is the physics, not the implementation. | Sprint 004 (LAN), Horizon B (WAN) |
| Activation size and KV-cache placement | Without a cache, activations grow with sequence length. With one, each stage holds state that is lost if the stage dies. | Sprint 003 → 004 |
| Partial failure | A dead stage mid-generation loses its KV cache. Resuming requires replicated state or recomputation. Supervision does not restore tensors. | Sprint 004 → 005+ |
| Heterogeneous scheduling | MPS vs CUDA, 24 GB vs 12 GB, different throughput per layer. Even stage splits are wrong. | Sprint 005+ |
| Trust, verification, incentives | How do you know a stranger ran the right layers? Open research. | Horizon B — deferred, not foreclosed |

## 11. Decisions

Recorded as ADRs (contested, or expected to be revisited):

- [ADR-0001 — Go core, not Elixir/OTP](adr/0001-go-core-not-elixir.md)
- [ADR-0002 — Hand-rolled transport behind a port](adr/0002-hand-rolled-transport-behind-port.md)
- [ADR-0003 — Go relays activations; no control/data split yet](adr/0003-go-relays-activations.md)

Small decisions not worth an ADR:

- Local Go↔Python IPC is loopback TCP, not Unix sockets (Windows portability;
  same framing; nothing measurable lost).
- Windows node runs natively, not under WSL2.
- `internal/` for all Go packages — nobody imports this as a library.
- nanoGPT/GPT-2 is the first model to partition; replaced by plan in Sprint 005.
- MIT license.

## 12. Open questions

Carried forward; answered by experiments, not by discussion.

- What is the minimum state that must be checkpointed to resume a pipeline
  after a stage dies — and where does it live?
- Where should the KV cache live when a stage can move between machines?
- At what point does request parallelism matter more than deeper pipelines?
- What does the relay cost (ADR-0003) actually look like as a fraction of
  per-token latency on a LAN?
