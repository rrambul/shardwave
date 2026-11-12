# Shardwave — notes for Claude

Shardwave is a learning project: an open-source P2P system for running one
large model across several machines. Renan is learning Go and distributed
systems by building it. That is the goal, not a product. Source of truth is
`docs/NORTH_STAR.md`; decisions are in `docs/adr/`; the current plan is in
`docs/sprints/`.

## How we work

- One package or coherent change per round. First the architecture context
  and the end-to-end story the piece takes part in, then the code block by
  block with the Go constructs named as they appear, then the review points.
  Then stop and wait for the review.
- Never commit, push, or open a PR without an explicit go-ahead for that
  change. Renan reviews first.
- Every new file, abstraction, or ADR needs a consumer or a contested
  decision behind it. When challenged on count, concede honestly.
- Learning beats speed. Explain the why, name the alternative that was not
  taken, and link the primary source (pkg.go.dev, go.dev/blog, the style
  guides) instead of paraphrasing it.

## Code

- Follow `docs/GO_STYLE.md`. It distills the Google Go Style Guide, Go Code
  Review Comments, and Effective Go into the rules this repo applies, and
  says which are enforced by the linter and which are checked in review.
- `make check` (vet, golangci-lint, `go test -race`) passes before code is
  handed over for review. CI runs the same.
- Comments are lean: a one-line doc comment on every exported identifier, an
  inline comment only for a *why* the code cannot say. Rationale goes in the
  walkthrough, the ADR, or the commit message.
- Standard library first. `golang.org/x` is acceptable. Anything else needs a
  reason in the commit message or an ADR.
- Never log or print a private key or seed. Log `PeerID.Short()`.

## Layout

- `cmd/shardwave` is the binary. Everything else is under `internal/` and is
  not importable from outside this module, on purpose.
- `internal/protocol` is transport-agnostic. Ports (`transport`, later
  `executor`) are small interfaces in the consumer's package; adapters live in
  subpackages (ADR-0002, ADR-0003).
