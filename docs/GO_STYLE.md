# Go style for Shardwave

The rules this repo applies, distilled from the sources below. Section 1 is
enforced by `make check`. Everything after it is what a review looks for,
because no linter can see it. Each rule names its source so you can read the
reasoning, which is usually better than the rule.

- [CRC] Go Code Review Comments — <https://go.dev/wiki/CodeReviewComments>
- [GSG] Google Go Style Guide — <https://google.github.io/styleguide/go/>
  (the *Decisions* and *Best practices* pages carry the why)
- [EG] Effective Go — <https://go.dev/doc/effective_go>
- [DOC] Go Doc Comments — <https://go.dev/doc/comment>
- [UBER] Uber Go Style Guide — <https://github.com/uber-go/guide/blob/master/style.md>

## 1. Enforced by tooling

- `gofmt`, `gofumpt`, and `goimports` decide formatting; nobody argues about it. [CRC]
- `go vet` and the linters in `.golangci.yml` run on every change. A `nolint`
  names the linter and gives a reason on the same line.
- `go test -race -count=1 ./...` is the only way tests run.
- Every exported identifier has a doc comment (revive `exported`). [DOC]

## 2. Naming

- `MixedCaps` or `mixedCaps`, never underscores. [EG]
- Package names: one short lowercase word, no `util`, `common`, `helpers`,
  `models`. The package name is part of every identifier's name, so
  `protocol.Frame`, not `protocol.ProtocolFrame`. [CRC] [GSG]
- Initialisms keep their case: `PeerID`, `peerID`, `HTTPServer`. Never `PeerId`. [CRC]
- Receiver names are one or two letters, the same for every method on the
  type, never `this` or `self`. [CRC]
- No `Get` prefix on getters. One-method interfaces end in `-er`. [EG]
- Name length scales with distance from the declaration: `i` in a loop, `peerTable` across a file. [GSG]
- Unexported by default. Export only what another package needs today.

## 3. Doc comments

- A full sentence that starts with the name: `// ReadFrame reads one frame from r.` [DOC]
- One package comment per package, in the most important file or `doc.go`. [DOC]
- A block of related constants or variables may share one comment on the block. [DOC]
- A method that implements a standard interface says so and nothing more:
  `// MarshalBinary implements encoding.BinaryMarshaler.`
- Inline comments explain a *why* the code cannot say. Rationale and
  alternatives go in the walkthrough, the ADR, or the commit message. (This repo.)

## 4. Errors

- Return errors; do not panic. Panic only for a programmer error that cannot
  happen on any input from the outside. [GSG]
- Error strings are lowercase, end without punctuation, and carry package
  context when they cross a package boundary: `protocol: unknown message type`. [CRC]
- Wrap with `%w` once, at the point that adds information. Do not wrap the
  same error at every level. [GSG]
- Compare with `errors.Is` and `errors.As`, never `==` or a type assertion on
  an error value. [Go blog, *Working with Errors in Go 1.13*]
- Sentinels are `ErrFoo`; error types are `FooError`. [CRC]
- Handle an error once: log it or return it, not both. [UBER]
- `_ = f()` only with a comment saying why the error does not matter. [CRC]
- No in-band errors (`-1`, `nil`, `""` meaning failure). Return an `error` or an `ok bool`. [CRC]

## 5. Context

- `ctx context.Context` is the first parameter, named `ctx`. [CRC]
- Never store a context in a struct. Pass it. [CRC]
- Never pass `nil`; use `context.TODO()` when unsure. [CRC]
- Every blocking loop selects on `ctx.Done()`. [Go blog, *Context*]

## 6. Interfaces

- Define an interface where it is consumed, not where it is implemented. [GSG]
- Keep it small. Two methods are better than five. [EG]
- Accept interfaces, return concrete types. [GSG]
- Do not add an interface before there is a second implementation or a test
  that needs one. The two ports from ADR-0002 and ADR-0003 are the deliberate
  exception; nothing else gets one for free. [GSG]
- Satisfying an interface is implicit. Do not write `var _ Foo = (*Bar)(nil)`
  unless the compile-time check is the point. [EG]

## 7. Receivers and values

- All methods on a type use the same receiver kind. [CRC]
- Pointer receiver if the method mutates, the struct is large, or it contains
  a `sync` type. Value receiver for small immutable types such as `PeerID`
  and `MsgType`. [CRC]
- Never copy a value that contains a `sync.Mutex` or `sync.WaitGroup`; `go vet copylocks` catches most cases. [GSG]

## 8. Concurrency

- Never start a goroutine without knowing how it stops and who waits for it.
  [Dave Cheney, *Never start a goroutine without knowing how it will stop*]
- The function that starts a goroutine owns its lifetime: `sync.WaitGroup` or
  `errgroup`, and `Close` returns only when every goroutine has exited. [GSG]
- Every goroutine takes a `ctx` and exits when it is cancelled. [Go blog, *Context*]
- Close a channel from the sender, never from a receiver. [Go blog, *Pipelines*]
- A mutex protects state; a channel hands ownership between goroutines.
  Pick by which one you are doing. [Go blog, *Share Memory By Communicating*]
- Channels do not appear in an exported API unless the API is about events. [GSG]
- Never `time.Sleep` to synchronize, in code or in tests. [GSG]
- Run `-race` every time. It finds races that happened, not races that can happen. [Go, *Data Race Detector*]

## 9. Zero values and construction

- Make the zero value useful. A `New` function exists only when an invariant
  needs establishing. [EG] [GSG]
- No `init()`. No mutable package-level state. Package-level `Err` sentinels
  and constants are fine. [GSG]
- Struct literals name their fields. [GSG]

## 10. Slices, maps, arrays

- A nil slice is a valid empty slice. Return `nil`, not `[]T{}`. [CRC]
- Never return a slice or map that aliases internal state. Copy. [GSG]
- `make` with capacity when the length is known. [Go blog, *Slices*]
- Arrays are values and comparable with `==`; slices are headers and are not.
  `copy(dst[:], src)` moves bytes into an array. [Go blog, *Slices*]

## 11. Testing

- Table-driven, with `t.Run` and a name per case. [GSG]
- Failure messages read `Func(x) = got, want want`. [CRC]
- `t.Helper()` is the first line of every helper. [GSG]
- `t.TempDir()`, `t.Context()`, `t.Setenv()`, never the `os` equivalents. [GSG]
- `t.Fatal` when the test cannot continue, `t.Error` when it can. [GSG]
- No assertion libraries. `if got != want` is the assertion library. [GSG]
- Test through the exported API unless the internals are the point. [GSG]
- Synchronize with channels or `sync`, never with sleeps. [GSG]

## 12. Dependencies and layout

- Standard library first. `golang.org/x` is acceptable. Anything else is
  justified in the commit message or an ADR. (This repo.)
- `go mod tidy` runs in every commit that touches `go.mod`.
- `internal/` for everything that is not the binary. Package by
  responsibility (`protocol`, `transport`), never by kind (`types`, `utils`).
  [go.dev/doc/modules/layout]
- Tests live next to the code in the same package. Use an external `_test`
  package only when the test should see the package as a user does. [GSG]

## 13. Logging

- `log/slog`, structured key/value pairs. [pkg.go.dev/log/slog]
- Libraries return errors; `cmd/` logs them. [UBER]
- Log `PeerID.Short()`. Never a private key, a seed, or a full payload.
