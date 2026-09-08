日本語版: [05-args-and-env_ja.md](05-args-and-env_ja.md)

# 5. Arguments and Environment Variables

`hello-wasi` also prints its own arguments (`os.Args`) and the environment
variable `GREETING`. Let's pass those in with `--` and `-e`.

```
$ ./hello-wasi -s out -e GREETING=konnichiwa -- foo bar
hello from execsandbox
args: [execsandbox foo bar]
GREETING=konnichiwa
```

- **Everything after `--`** becomes the guest's arguments. `foo` and `bar`
  are passed straight through into `os.Args[1:]`. `os.Args[0]`, though,
  isn't the guest's own file name — it's the fixed string `"execsandbox"`,
  because the embedded WASM module has no notion of a "file name" to
  begin with.
- **`-e KEY=VALUE`** (repeatable) shows the guest only the environment
  variables you specify. The host shell's own environment variables are
  never inherited as-is. `GREETING` stayed empty without `-e` (see the
  previous chapter).

By this point (chapters 3 through 5), ExecSandbox's basic stance should be
coming into focus. **Even things an ordinary program would take for
granted — standard output, arguments, environment variables — are
unusable unless explicitly allowed.** That's what capability-based policy
feels like in practice.

Starting with the next chapter, we move beyond a single sandbox and into
**connecting multiple sandboxes** together.

---
[← Previous: 4. Seeing Output](04-seeing-output.md) | [Index](README.md) | [Next: 6. The Mailbox Idea →](06-the-mailbox-idea.md)
