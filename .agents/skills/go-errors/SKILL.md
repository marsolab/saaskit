---
name: go-errors
description: >-
  Idiomatic Go error handling. ALWAYS use this skill when writing or
  reviewing Go code that raises, returns, wraps, matches, or aggregates
  errors — questions about `fmt.Errorf` with `%w` vs `%v`, `errors.Is` and
  `errors.As`, sentinel errors, typed error structs, `errors.Join` for
  combining failures, the "handle exactly once" rule, panic vs error,
  guard clauses and early return, error message formatting, or any
  `if err != nil` pattern. Use alongside go-style for general idioms.
when_to_use: >-
  TRIGGER WHEN the user is raising, returning, wrapping, matching, or
  aggregating Go errors — `fmt.Errorf` with `%w`, `errors.Is`,
  `errors.As`, `errors.Join`, sentinel errors (`var ErrFoo = ...`),
  typed error structs implementing `Error() string`, the "handle
  exactly once" rule, log-or-return decisions, panic vs error in
  libraries, guard clauses, early return, lowercased no-punctuation
  error messages, exit-code mapping from typed errors, or any
  `if err != nil { ... }` review question. ALSO TRIGGER on phrases
  like "wrap this error", "add error context", "join these errors",
  "match a sentinel". SKIP for non-Go languages.
version: 1.2.0
tags:
  - go
  - golang
  - errors
  - error-handling
  - errors-join
paths:
  - "**/*.go"
---

# Go Errors

Errors are values. Treat them with the same care as any other return.

For the comprehensive reference, see `references/error-handling.md`.

## The four rules

1. **Check every returned error.** No exceptions for "this can't fail."
   That includes `Close`, `Flush`, and `Write` in a `defer` — see
   "Deferred cleanup" below.
1. **Handle each error exactly once.** Either log it OR return it; never
   both. Log-and-return causes duplicate log lines further up the stack.
1. **Wrap with context as the error travels up.**
1. **Never panic in library code.** Return an error and let the caller
   decide.

## Decision tree

```
Got an error from a call.
├── Can I handle it completely here?            → Log (or recover) and continue.
├── Does the caller need programmatic match?    → fmt.Errorf("op: %w", err) — wraps.
├── Should I hide implementation details?       → fmt.Errorf("op: %v", err) — flattens.
└── Is this a library?                          → Return; never log.
```

## Wrapping

```go
// Add context as the error climbs the stack
if err != nil {
    return fmt.Errorf("connect to database: %w", err)
}
```

- Lowercase, no trailing punctuation, no `failed to` prefix. The strings
  get concatenated: `processing request: connect to database: connection refused`
  reads cleanly.
- `%w` wraps so `errors.Is` / `errors.As` work upstream.
- `%v` flattens — use when you don't want callers depending on the inner
  error's type.

## Sentinels and typed errors

```go
// Sentinel: comparable with errors.Is
var ErrNotFound = errors.New("user not found")

// Typed error: comparable with errors.As, carries data
type ValidationError struct {
    Field string
    Issue string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s: %s", e.Field, e.Issue)
}

// Caller side
var verr *ValidationError
switch {
case errors.Is(err, ErrNotFound):
    // 404
case errors.As(err, &verr):
    // 400 with verr.Field
}
```

## errors.Join — combining failures

Use when several independent operations may each fail and you want to
report all of them:

```go
func ValidateConfig(cfg Config) error {
    var errs []error
    if cfg.Host == "" {
        errs = append(errs, errors.New("host is required"))
    }
    if cfg.Port < 1 || cfg.Port > 65535 {
        errs = append(errs, fmt.Errorf("invalid port: %d", cfg.Port))
    }
    return errors.Join(errs...) // nil if errs is empty
}
```

`errors.Join` returns `nil` for an empty slice and supports `errors.Is` /
`errors.As` against any constituent. Also handy for cleanup paths:

```go
func cleanup(db *sql.DB, file *os.File) error {
    return errors.Join(db.Close(), file.Close())
}
```

## Deferred cleanup: capture the Close error

`Close`, `Flush`, and `Write` return errors that matter — a failed
`Close` can mean buffered data never reached disk or the network. Rule 1
applies. Don't discard the error with `_ =`, and don't suppress the
`errcheck` lint with `//nolint`.

When the resource is opened after the guard clauses, name the returns and
fold the cleanup error in from the `defer`:

```go
// GOOD — the named return lets the defer report Close's failure
func Ping(ctx context.Context, addr string) (pong Pong, rErr error) {
    conn, err := (&net.Dialer{}).DialContext(ctx, "udp", addr)
    if err != nil {
        return Pong{}, fmt.Errorf("dial %q: %w", addr, err)
    }
    defer func() {
        if err := conn.Close(); err != nil {
            rErr = errors.Join(rErr, fmt.Errorf("close conn: %w", err))
        }
    }()

    // ... use conn; a mid-body `return Pong{}, fmt.Errorf(...)` still runs
    // the defer, so a close failure is joined onto whatever you returned.
}
```

`errors.Join` keeps the body's error and appends the close failure; if
both are nil the result is nil. This is the only form that surfaces a
cleanup failure without swallowing the original error.

```go
// BAD — discards a real error and silences the linter to do it
defer func() { _ = conn.Close() }() //nolint:errcheck
```

Use the deferred handler everywhere, even for a read-only file whose
`Close` almost never fails: it costs nothing, keeps you from deciding
case by case, and satisfies the bundled `errcheck` config, which flags
both `_ = f.Close()` (via `check-blank`) and a bare `f.Close()`. Writers,
network connections, and anything that flushes must always have `Close`
checked.

## Guard clauses keep the success path left

```go
// GOOD
func ProcessFile(path string) (rErr error) {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("open %s: %w", path, err)
    }
    defer func() {
        if err := f.Close(); err != nil {
            rErr = errors.Join(rErr, fmt.Errorf("close %s: %w", path, err))
        }
    }()

    data, err := io.ReadAll(f)
    if err != nil {
        return fmt.Errorf("read %s: %w", path, err)
    }
    return process(data)
}
```

Avoid `else` branches after a `return`. Avoid nested error checks — flat
is readable. The deferred closure handles the `Close` error per
"Deferred cleanup" above without pushing the success path right.

## Don't panic

Panics crash the program and cascade through any process running the
library. Return an error every time.

Acceptable panic uses:

- API misuse inside internal code (e.g. `reflect`-style developer error).
- `main()` initialization failures — prefer `log.Fatal` over `panic` so
  deferred functions don't deadlock.
- Marking unreachable code paths.

## When to load a sibling skill

| Task | Skill |
|---|---|
| `errgroup` patterns, goroutine error propagation | go-concurrency |
| Logging an error you've decided to handle | go-logging |
| Returning HTTP status codes from typed errors | go-http |
| Wrapping `sql.ErrNoRows` etc. at the data layer | go-sql |
| General Go idioms and naming | go-style |
