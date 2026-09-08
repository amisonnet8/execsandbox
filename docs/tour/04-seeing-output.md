# 4. Seeing Output

Let's add `-s out` (`--stdio out`) to the previous chapter's `hello-wasi`
and connect standard output to the host side.

```
$ ./hello-wasi -s out
hello from execsandbox
args: [execsandbox]
GREETING=
```

Same guest, same code, and yet just adding `-s out` made the output appear.
`-s` is the option that chooses which of the guest's streams get connected
to the outside — `in`/`out`/`err`/`all`, comma-separated (everything is
blocked by default).

`args` shows only `[execsandbox]`, and `GREETING=` is empty, because we
haven't passed any arguments or environment variables yet. That comes in
the next chapter.

## A small note: the host's logs and the guest's output are two different things

ExecSandbox itself sometimes emits diagnostic logs too, but those are
written to standard error, prefixed with `execsandbox:`. What `-s out`
makes visible is strictly the standard output **the guest itself wrote**.
Keeping these two apart is the trick to reading the logs correctly.

---
[← Previous: 3. Nothing by Default](03-nothing-by-default.md) | [Index](README.md) | [Next: 5. Arguments and Environment Variables →](05-args-and-env.md)

日本語版: [04-seeing-output_ja.md](04-seeing-output_ja.md)
