# 14. Allowing Randomness and Time

Every chapter so far has followed "deny by default, allow explicitly," and
randomness and time are no exception — `-a/--allow` is what opts them in.
The guest below fetches one byte of randomness and the current time, and
prints both.

```
$ tinygo build -target=wasip1 -o allow.wasm .
$ execsandbox-build -o allow allow.wasm
$ ./allow -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
$ ./allow -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
```

By default, the time is pinned to the fake clock at 2022-01-01T00:00:00Z.
The randomness side doesn't turn into an "error" either — it comes back as
**the same fixed value, `117`, on every run.** This looks strange, but it
isn't a bug in ExecSandbox itself. The host reliably denies at the ABI
boundary (`random_get`), but the TinyGo `crypto/rand` this guest uses
doesn't call WASI's `random_get` directly — it goes through a libc function
called `arc4random_buf`. By C convention, this function has no way to
propagate a failure back to its caller. As a result, `random_get`'s error
gets swallowed, and the guest side observes it as "the same value comes
back every time."

**Even when the host denies something reliably, how the guest's runtime
handles the resulting error downstream is entirely up to the guest
language's implementation** — that's one lesson of this chapter.

With `-a random,time`, both become real.

```
$ ./allow -a random,time -s out
random byte: 174
clock: 2026-09-09T00:44:49Z
$ ./allow -a random,time -s out
random byte: 128
clock: 2026-09-09T00:44:49Z
```

---
[← Previous: 13. Exposing Files](13-exposing-files.md) | [Index](README.md) | [Next: 15. Writing in Another Language →](15-writing-in-another-language.md)

日本語版: [14-allowing-random-and-time_ja.md](14-allowing-random-and-time_ja.md)
