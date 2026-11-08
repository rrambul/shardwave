# ADR-0003 — Go relays activations; no control/data-plane split yet

- **Status:** Accepted
- **Date:** 2026-11-08

## Context

Pipeline-parallel inference moves activation tensors between stages. The Go
node orchestrates; Python executes. Three ways to connect them:

1. **Go relays.** The Go node spawns its Python worker; they share the
   `protocol` framing over a local socket. Activations flow
   `Python A → Go A → network → Go B → Python B`.
2. **Control/data split.** Go negotiates only; Python workers open direct
   connections to each other for tensors. Go never sees an activation. This is
   what production systems converge on.
3. **gRPC between Go and Python.** Typed and streaming, but adds protobuf
   tooling and hides the framing we are learning to write.

## Decision

**Option 1.** Go relays activations through Sprint 004.

- `executor.Executor` is the port:
  `Execute(ctx, stage StageID, input Tensor) (Tensor, error)`.
- Adapter #0 is a Go mock (matmul) so the node is testable without Python.
- Adapter #1 spawns a Python subprocess and talks to it over **loopback TCP
  (`127.0.0.1:port`)** using the same `protocol` framing as node-to-node.
  Loopback TCP rather than Unix sockets because the Windows node runs natively
  and CPython's Unix-socket support on Windows is unreliable; the cost on
  loopback is not measurable.
- The Python worker has no networking beyond that loopback socket. It
  receives a tensor, runs `forward()` over its layers, returns a tensor.
- Tensor payload: small header (dtype, shape, stage id) followed by raw bytes,
  safetensors-style. **Never pickle** — Python-only and unsafe at Horizon B.
- The Windows node runs natively (`GOOS=windows`), not under WSL2, whose
  virtual NAT breaks inbound LAN connections and mDNS.

## Reasons

- Consistent with "Go is the brain": the orchestrator sees every byte and can
  instrument, log, retry, and measure in one place.
- **One wire protocol.** The Sprint 001 frame is reused for local IPC. Nothing
  learned twice.
- Python stays dumb and portable.
- The cost is real and that is the point: Sprint 004 measures relay overhead as
  a fraction of per-token latency. The control/data split then becomes an ADR
  justified by a number, not by reading about it.

## Consequences

- Two extra memory copies per activation hop (Python → Go, Go → Python).
- Per-token latency includes two loopback round trips per stage.
- Option 2 would mean Python bypassing the transport port entirely; deferring
  it keeps the whole system behind the two ports during the learning phase.

## Revisit when

- Sprint 004 shows relay overhead is a significant fraction of per-token
  latency (threshold to be set when we have the first number — a plausible
  line is >15%).
- A Python worker needs to talk to more than one peer directly.
- Activation sizes make the double copy a memory problem rather than a latency
  one.
