# 11. Stopping a Runaway Guest

From here on, we move into four chapters that use no SDK at all, looking
only at ExecSandbox's own policy features. First up: `-t/--timeout` — the
execution time limit.

The guest below doesn't even call `Recv`; it's a pure infinite loop that
checks nothing whatsoever.

```go
func main() {
	for {
	}
}
```

```
$ tinygo build -target=wasip1 -o timeout.wasm .
$ execsandbox-build -o timeout timeout.wasm
$ time ./timeout -t 1s
execsandbox: execution timed out after 1s

real	0m1.010s
```

With `-t 1s` given, the guest is reliably force-killed after one second,
even though it checks nothing at all. The exit code is `124` (the same
convention as Unix's `timeout(1)` command). Adding `-q` suppresses this
log line, but the exit code doesn't change.

For a sandbox running an untrusted guest, the guarantee that "it can be
stopped even if the guest doesn't cooperate" is an important property.

---
[← Previous: 10. Telling Events Apart](10-telling-events-apart.md) | [Index](README.md) | [Next: 12. Capping Memory →](12-capping-memory.md)

日本語版: [11-stopping-a-runaway-guest_ja.md](11-stopping-a-runaway-guest_ja.md)
