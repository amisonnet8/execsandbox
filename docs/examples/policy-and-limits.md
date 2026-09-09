# policy-and-limits — resource limits and capabilities

Demonstrates `-v` (file mounting), `-m` (memory limit), `-t` (timeout), and
`-a` (allowing randomness/time), each with a minimal TinyGo guest. No SDK is
used, so as to show only ExecSandbox's own policy features.

Source: [`src/policy-and-limits/`](src/policy-and-limits/) (four independent
guests: `file-access/`, `mem-limit/`, `timeout/`, `allow/`)

## `-v, --volume` — file access

[`file-access/main.go`](src/policy-and-limits/file-access/main.go) writes to
`/data/hello.txt` (right under the guest-side mount point) and reads it
back.

```
$ tinygo build -target=wasip1 -o file-access.wasm .
$ execsandbox-build -o file-access file-access.wasm
```

By default, without `-v`, the access path to the filesystem doesn't exist
at all.

```
$ ./file-access -s out
write failed: open /data/hello.txt: file does not exist
```

Mounting a host directory with `-v HOST:/data` allows writing.

```
$ mkdir hostdata
$ ./file-access -v "$PWD/hostdata:/data" -s out
write ok
read back: written by the guest
$ cat hostdata/hello.txt
written by the guest
```

## `-m, --mem-limit` — memory limit

[`mem-limit/main.go`](src/policy-and-limits/mem-limit/main.go) keeps
allocating slices 1MiB at a time, holding on to everything it manages to
allocate (to prevent the GC from reclaiming them and never hitting the
limit).

```
$ tinygo build -target=wasip1 -o mem-limit.wasm .
$ execsandbox-build -o mem-limit mem-limit.wasm
$ ./mem-limit -m 16M -s out
allocated 1 MiB so far
...
allocated 12 MiB so far
fatal error: out of memory
execsandbox: run WASM module: module[main] function[_start] failed: wasm error: unreachable
```

Once TinyGo's own runtime hits the ceiling of its allocation region (linear
memory), it exits the guest with `fatal error: out of memory`. It runs out
around 12MiB even with `-m 16M` because TinyGo's runtime itself and the
GC's metadata already occupy part of linear memory. **The host process
itself doesn't crash; this is treated as an abnormal exit of the guest**
(exit code 1, an `execsandbox:`-prefixed log line).

## `-t, --timeout` — execution time limit

[`timeout/main.go`](src/policy-and-limits/timeout/main.go) doesn't even
call `recv` — a pure infinite loop that checks nothing at all (the
simplest form of what `.claude/rules/wazero-quirks.md` calls "an infinite
loop that completes entirely outside any host function call").

```
$ tinygo build -target=wasip1 -o timeout.wasm .
$ execsandbox-build -o timeout timeout.wasm
$ time ./timeout -t 1s
execsandbox: execution timed out after 1s

real	0m1.010s
```

The exit code is `124` (the same convention as Unix's `timeout(1)`
command). Adding `-q` suppresses this log line, but the exit code doesn't
change.

## `-a, --allow` — allowing randomness and time

[`allow/main.go`](src/policy-and-limits/allow/main.go) fetches one byte of
randomness via `crypto/rand` and the current time via `time.Now()`, and
prints both.

```
$ tinygo build -target=wasip1 -o allow.wasm .
$ execsandbox-build -o allow allow.wasm
```

By default (no `-a`), **the random byte comes back as the same value,
`117`, on every run** (denied in the sense that it's no longer truly random,
but this doesn't surface as an error), and **the time is pinned to the fake
clock at 2022-01-01T00:00:00Z**, as `docs/usage/execsandbox.md` explains.

```
$ ./allow -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
$ ./allow -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
```

With `-a random,time`, the random byte changes on every run, and the time
matches the real current time.

```
$ ./allow -a random,time -s out
random byte: 174
clock: 2026-09-09T00:44:49Z
$ ./allow -a random,time -s out
random byte: 128
clock: 2026-09-09T00:44:49Z
```

### Why denied randomness becomes a fixed value instead of an "error"

Per spec §5.6 and `.claude/rules/wazero-quirks.md`'s design, denying
randomness (the default, no `-a random`) always blocks the `random_get` call
itself with an error. However, **TinyGo's `crypto/rand` (on the `wasip1`
target) doesn't call WASI's `random_get` directly — it goes through a libc
function called `arc4random_buf`**. By C convention, this function has no
return value and no way to propagate a failure to its caller. As a result,
even when `random_get` fails internally, `arc4random_buf` swallows it (this
appears to depend on TinyGo's wasi-libc implementation returning an
uninitialized buffer as-is, producing the same value every time), and the
guest side observes it as "the same value comes back every time."

**This isn't a bug in ExecSandbox itself — it's a behavior stemming from the
guest language's libc implementation.** Denying randomness reliably blocks at
the host's ABI boundary (`random_get`), but this is a live example showing
that how the guest runtime handles the resulting error is entirely up to the
guest's own implementation. Behavior may differ for the Rust SDK or a guest
that calls the raw ABI directly.

---

日本語版: [policy-and-limits_ja.md](policy-and-limits_ja.md)
