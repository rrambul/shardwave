# ADR-0001 — Go core, not Elixir/OTP

- **Status:** Accepted
- **Date:** 2026-11-08

## Context

Elixir/OTP was proposed twice as Shardwave's coordination core:

1. As the primary stack, with `libp2p_elixir` for networking and a
   ports-and-adapters layer allowing Go or Rust libp2p *sidecars* later.
2. As an "experimental coordination core" justified by a research hypothesis:
   *"Can OTP-based coordination improve recovery and execution continuity
   during distributed model-parallel inference?"*

The genuine argument for OTP is real: supervision trees, process isolation,
and monitors are an elegant way to express failure detection and restart
policy. Go requires building these explicitly.

## Decision

**Go is the core.** Peer identity, discovery, transport, peer table,
liveness, orchestration, and relay are all Go. There is no Elixir in the
project. Supervision, restart policy, and failure detection are implemented
in Go with goroutines and `context`, at the moment the system first needs them.

## Reasons

1. **Learning priorities.** The ranked goals are (1) distributed systems in
   Go, (2) ML systems, (3) fault-tolerant coordination. Go is both the tool
   and a learning target. OTP ranks last.
2. **The hypothesis is not testable here.** "Improve" needs a non-OTP baseline
   doing the same recovery. A solo learner with one stack will not build the
   baseline. It is a preference, not an experiment — which is fine, but it
   should not be dressed as research.
3. **The hard part of recovery is language-agnostic.** OTP does not restore a
   dead worker's KV cache or recompute lost activations. Detecting the dead
   stage is trivial in any runtime; deciding what state to replicate and how
   to resume is ML-systems work. OTP buys ergonomics around the state machine,
   not the state machine.
4. **Three runtimes for one learner.** Elixir core + Go sidecar + Python
   worker means three languages, three build/test systems, two IPC contracts.
5. **The library is immature.** `libp2p_elixir` released v0.9.0–0.9.6 between
   2025-12-18 and 2025-12-26 and is not on libp2p's official implementations
   list. Betting on it most likely lands on the sidecar fallback anyway.
6. **The sidecar architecture is slow by construction.** An activation would
   traverse `Python → Elixir → Go → TCP → Go → Elixir → Python`: four IPC hops
   per stage, before any model runs. The proposal's own "performance awareness"
   principle argues against it.
7. **Supervision in Go is small.** A restart-with-backoff supervisor is on the
   order of 100 lines. Learning it by building it serves goal (3) without a
   third language.

## Consequences

- We lose OTP's built-in supervision ergonomics and write our own.
- Fault-tolerance design is deferred until Sprint 004 produces concrete
  failure modes to design against.
- The Go ↔ Python boundary is the only IPC contract in the system.

## Revisit when

- A second maintainer joins whose primary goal is OTP research — and then as a
  *separate experiment* comparing recovery strategies, not a rewrite of the
  core.
- Otherwise: not within Horizons C or B.
