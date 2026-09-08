日本語版: [14-denying-random-and-time_ja.md](14-denying-random-and-time_ja.md)

# 14. Denying Randomness and Time

Every chapter so far has followed "deny by default, allow explicitly."
`-x/--deny` is the sole exception — **randomness and time are allowed by
default**, and `-x` denies them explicitly. The guest below fetches one
byte of randomness and the current time, and prints both.

```
$ tinygo build -target=wasip1 -o deny.wasm .
$ execsandbox-build -o deny deny.wasm
$ ./deny -s out
random byte: 158
clock: 2026-09-08T16:02:50Z
$ ./deny -s out
random byte: 87
clock: 2026-09-08T16:02:50Z
```

With `-x random,time`, the time is pinned to the fake clock at
2022-01-01T00:00:00Z.

```
$ ./deny -x random,time -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
$ ./deny -x random,time -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
```

The randomness side doesn't turn into an "error" — it becomes **the same
fixed value, `117`, on every run.** This looks strange, but it isn't a bug
in ExecSandbox itself. The host reliably blocks at the ABI boundary
(`random_get`), but the TinyGo `crypto/rand` this guest uses doesn't call
WASI's `random_get` directly — it goes through a libc function called
`arc4random_buf`. By C convention, this function has no way to propagate a
failure back to its caller. As a result, `random_get`'s error gets
swallowed, and the guest side observes it as "the same value comes back
every time."

**Even when the host blocks something reliably, how the guest's runtime
handles the resulting error downstream is entirely up to the guest
language's implementation** — that's the lesson of this chapter.

---
[← Previous: 13. Exposing Files](13-exposing-files.md) | [Index](README.md) | [Next: 15. Writing in Another Language →](15-writing-in-another-language.md)
