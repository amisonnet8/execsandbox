日本語版: [README_ja.md](README_ja.md)

# Examples

Shows what ExecSandbox can do, with working code and actual command output.
Each one is self-contained, and the reading order doesn't matter. For the
list and syntax of options, see [`docs/usage/`](../usage/); for the
rationale behind design decisions, see [`docs/spec/`](../spec/).

| Example | What it shows | Launch options used |
| :--- | :--- | :--- |
| [`hello-wasi.md`](hello-wasi.md) | A minimal setup with no SDK | `-s`, `-e`, `--` |
| [`sandbox-messaging.md`](sandbox-messaging.md) | Sandbox-to-sandbox messaging | `-n`, `-d` |
| [`external-connection.md`](external-connection.md) | Echoing back an external connection | `-l` |
| [`policy-and-limits.md`](policy-and-limits.md) | File access, memory limit, timeout, denying randomness/time | `-v`, `-m`, `-t`, `-x` |
| [`polyglot-messaging.md`](polyglot-messaging.md) | Connecting a TinyGo guest and a Rust guest | `-n`, `-d` |

The guest source for `hello-wasi.md` and `policy-and-limits.md` lives in
[`src/`](src/). `sandbox-messaging.md`, `external-connection.md`, and
`polyglot-messaging.md` use the examples from the
[`execsandbox-sdk`](https://github.com/amisonnet8/execsandbox-sdk)
repository as-is.

## Prerequisites

Every example assumes you have a toolchain to build the guest (either
[TinyGo](https://tinygo.org/) or [Rust](https://www.rust-lang.org/), or
both), and `execsandbox-build` on hand. For how to obtain the builder
itself, see [`README.md`](../../README.md) at the repository root.
