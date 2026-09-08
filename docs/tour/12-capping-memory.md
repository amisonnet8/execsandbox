日本語版: [12-capping-memory_ja.md](12-capping-memory_ja.md)

# 12. Capping Memory

`-m/--mem-limit` sets the ceiling on the guest's WASM linear memory. The
guest below keeps allocating slices 1MiB at a time, holding on to
everything it allocates (to keep the GC from reclaiming them and never
hitting the limit).

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

It runs out around 12MiB even with `-m 16M`, because TinyGo's own runtime
and the GC's metadata already occupy part of linear memory. **The host
process itself doesn't crash; this is treated as an abnormal exit of the
guest** (exit code 1, an `execsandbox:`-prefixed log line) — like `-t` in
chapter 11, this is one form of the guarantee that a runaway guest never
takes the whole host down with it.

---
[← Previous: 11. Stopping a Runaway Guest](11-stopping-a-runaway-guest.md) | [Index](README.md) | [Next: 13. Exposing Files →](13-exposing-files.md)
