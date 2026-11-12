# Sprint 001 — P2P Foundation

**Goal:** two Go nodes establish an authenticated connection and keep it
alive, on this Mac and then across the LAN to the Windows PC.

**Definition of done:** [North Star §8, Sprint 001](../NORTH_STAR.md#sprint-001--p2p-foundation).
Not repeated here.

---

## How this sprint works

For each package, Claude explains **what** it does and **how** — the design
choices, the alternatives, the Go behavior that matters — then writes the
code and tests, then Renan reviews. The explanation is where the learning
happens; the review is where it is checked.

1. **Explain → write → review.** One package per round. Nothing moves on
   until the review is done.
2. **Tests are written with the code**, test-first where it clarifies the
   contract. `make test` runs with `-race` always on.
3. **Clean before review:** `make check` passes (vet, golangci-lint, tests
   with `-race`). CI runs the same on Linux and Windows.
4. **One commit per reviewed package.** Renan commits, or asks for it.
5. **Comments are lean.** A one-line doc comment on each exported name; an
   inline comment only for a *why* the code cannot say. The reasoning lives
   in the walkthroughs and the ADRs, not in the source.

(A coaching mode — Renan writes, Claude reviews — was tried first and
dropped on 2026-11-10.)

## Build order

| # | Package | What it is | What the tests pin | Go worth noticing |
|---|---|---|---|---|
| 1 | `internal/peer` | ed25519 keypair, `peer.ID = sha256(pub)`, save/load | distinct IDs; sign/verify; save/load round-trip; corrupt key never overwritten | fixed-size array as comparable type, `%w` + `errors.Is`, `t.Helper`, `t.TempDir` |
| 2 | `internal/protocol` | frame `[u32 len][u8 type][payload]`; Hello/HelloAck/Ping/Pong encoding | exact wire bytes; `io.EOF` vs `io.ErrUnexpectedEOF`; length checked before allocation; nothing written on error | `encoding/binary`, `io.ReadFull`, `BinaryMarshaler`, value vs pointer receivers |
| 3 | `internal/transport` | the port: `Transport`, `Discovery`, `PeerEvent` — from ADR-0002 | compiles; nothing else | what a port *is* in Go: a small interface in the consumer's package |
| 4 | `internal/transport/tcp` | the adapter: listen/dial, symmetric handshake, ping/pong liveness, events, shutdown | authenticated connect; bad signature rejected; round-trip; disconnect detected; `Close` leaks no goroutines | **the point of the sprint:** goroutine ownership, channels, `context`, `net.Conn` deadlines, `sync`, the race detector |
| 5 | `internal/discovery/static` | peers from a flag list | returns what it was given | the pattern, not the code |
| 6 | `internal/node` | peer table keyed by `peer.ID`; wires discovery → transport; lifecycle | state transitions; shutdown order | composing components; who owns which goroutine |
| 7 | `cmd/shardwave` | `node --listen --peers --key`; Ctrl-C → clean exit | manual: two terminals | `flag`, `signal.NotifyContext`, `log/slog` |
| 8 | cross-machine | `make windows`, copy the `.exe`, same handshake over the LAN | manual | `GOOS`/`GOARCH`, firewalls, reality |

`make build`, `run-a`, `run-b`, `windows` fail until step 7 exists.

---

## Reading list

Optional background per step, for reviewing with understanding. Links are to
primary sources; the standard library docs are the best Go documentation
there is.

### Foundations — do first if Go is new to you (about a day)

- **A Tour of Go** — <https://go.dev/tour> — all of it. Linger on *Methods and
  interfaces* and *Concurrency*.
- **Learn Go with Tests** — <https://quii.gitbook.io/learn-go-with-tests> —
  the *Go fundamentals* section. It teaches Go exactly the way this sprint
  runs: test first, then code.
- **Effective Go** — <https://go.dev/doc/effective_go> — skim now, re-read
  after step 4 when it will make sense.

### Always open

- **`docs/GO_STYLE.md`** — the rules this repo applies, each with its
  source. Review every package against it.
- **Go Code Review Comments** — <https://go.dev/wiki/CodeReviewComments> —
  the checklist Go reviewers actually use. Thirty short items.
- **Google Go Style Guide** — <https://google.github.io/styleguide/go/> —
  read *Decisions* for the why behind each rule.
- **Go Doc Comments** — <https://go.dev/doc/comment> — what a doc comment
  is and how `go doc` renders it.
- **How to Write Go Code** — <https://go.dev/doc/code> — modules, packages,
  `go test`, `go vet`.
- **Organizing a Go module** — <https://go.dev/doc/modules/layout> — why
  `cmd/` and `internal/` look the way they do.
- **Go Proverbs** — <https://go-proverbs.github.io/> — one line each. You will
  meet every one of them in this sprint.
- **Standard library reference** — <https://pkg.go.dev/std>.

### Step 2 — `protocol`

- `encoding/binary` — <https://pkg.go.dev/encoding/binary> —
  `BigEndian.PutUint32`, `BigEndian.Uint32`, `BigEndian.AppendUint64`.
- `io` — <https://pkg.go.dev/io> — `ReadFull` and, crucially, when it returns
  `io.EOF` versus `io.ErrUnexpectedEOF`. One test depends on you noticing that
  a zero-byte read *after a header* must become `ErrUnexpectedEOF`.
- `encoding` — <https://pkg.go.dev/encoding> — `BinaryMarshaler` /
  `BinaryUnmarshaler`, the standard interfaces your messages implement.
- *Working with Errors in Go 1.13* — <https://go.dev/blog/go1.13-errors> —
  `%w`, `errors.Is`, sentinel errors. The tests use `errors.Is` everywhere.
- *Go Slices: usage and internals* — <https://go.dev/blog/slices-intro> —
  why `copy(id[:], b)` and why arrays compare with `==` but slices do not.
- Dave Cheney, *Prefer table driven tests* —
  <https://dave.cheney.net/2019/05/07/prefer-table-driven-tests> — the shape
  every test in this repo uses.

### Steps 3–4 — `transport`, `transport/tcp`

Read before step 4, then again after your first green version.

- `net` — <https://pkg.go.dev/net> — `Listen`, `Dial`, `Conn`,
  `SetReadDeadline`. For tests: `net.Listen("tcp", "127.0.0.1:0")` picks a
  free port.
- `context` — <https://pkg.go.dev/context> and *Go Concurrency Patterns:
  Context* — <https://go.dev/blog/context>. Every goroutine you start takes a
  `ctx` and exits when it is done.
- *Go Concurrency Patterns: Pipelines and cancellation* —
  <https://go.dev/blog/pipelines> — the done-channel pattern, before `context`
  existed, so you see what `context` replaced.
- `sync` — <https://pkg.go.dev/sync> — `WaitGroup`, `Mutex`. `time` —
  <https://pkg.go.dev/time> — `Ticker`, `Timer`, `AfterFunc`.
- Dave Cheney, *Never start a goroutine without knowing how it will stop* —
  <https://dave.cheney.net/2016/12/22/never-start-a-goroutine-without-knowing-how-it-will-stop>
  — the one rule. The `Close` test enforces it.
- *Share Memory By Communicating* — <https://go.dev/blog/codelab-share>.
- **Data Race Detector** — <https://go.dev/doc/articles/race_detector> —
  `make test` runs with `-race`. Read what it can and cannot catch.
- Bryan C. Mills, *Rethinking Classical Concurrency Patterns* (GopherCon
  2018) — <https://www.youtube.com/watch?v=5zXAHh5tJqQ> — **watch after your
  first version of step 4 works.** It will tell you what to change, and you
  will understand it because you just made the mistakes it describes.
- `golang.org/x/sync/errgroup` — <https://pkg.go.dev/golang.org/x/sync/errgroup>
  — possibly the one dependency worth taking. Decide during step 4, not before.

### Step 7 — `cmd/shardwave`

- `flag` — <https://pkg.go.dev/flag>.
- `os/signal` — <https://pkg.go.dev/os/signal> — `signal.NotifyContext` turns
  Ctrl-C into context cancellation; the whole program unwinds from there.
- `log/slog` — <https://pkg.go.dev/log/slog> — structured logging. Log the
  short peer ID (`ID.Short()`); never log a private key.

### Cryptography used in steps 1 and 4

- `crypto/ed25519` — <https://pkg.go.dev/crypto/ed25519>, `crypto/rand` —
  <https://pkg.go.dev/crypto/rand>, `crypto/sha256` —
  <https://pkg.go.dev/crypto/sha256>.
- Why the handshake signs `domain ‖ nonce` and not the bare nonce: the doc
  comment on `protocol.HandshakeDigest`.

### Later, not now

- *100 Go Mistakes and How to Avoid Them* (Teiva Harsanyi) — after this
  sprint, when you have made twenty of them.
- *The Go Memory Model* — <https://go.dev/ref/mem> — the first time `-race`
  reports something you do not understand.

---

## Progress

- [x] 1 `peer` (was `identity`; renamed 2026-11-12 to drop the `identity.PeerID` stutter)
- [x] 2 `protocol` — awaiting review
- [ ] 3 `transport` (port)
- [ ] 4 `transport/tcp`
- [ ] 5 `discovery/static`
- [ ] 6 `node`
- [ ] 7 `cmd/shardwave`
- [ ] 8 cross-machine run
