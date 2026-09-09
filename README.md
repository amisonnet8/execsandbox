# ExecSandbox

A portable, zero-setup sandboxed execution tool that bundles a WASM
runtime and a WASM module into a single executable file. It features
capability-based policy and Erlang-style mailbox messaging.

## Installation

Download `execsandbox-build_<tag>_<GOOS>_<GOARCH>` (`.exe` for Windows
only) matching your environment from
[Releases](https://github.com/amisonnet8/execsandbox/releases). No Go
toolchain required.

```
chmod +x execsandbox-build_*
```

A `.sha256` file is bundled alongside it, so it's a good idea to verify
before using it.

```
sha256sum -c execsandbox-build_*.sha256
```

## Quick Start

```
# Embed a WASM module into a single executable
./execsandbox-build -o mydb mymodule.wasm

# Launch it. Nothing is allowed by default (filesystem, network,
# environment variables, and so on are all blocked unless explicitly
# permitted at launch).
./mydb -s out -- hello
```

## Documentation

- [Interactive guide (Gemini Notebook)](https://notebook.google.com/notebook/70022dee-dbf2-4365-af70-ad98b28a613f) —
  an AI-generated interactive walkthrough
- [`docs/tour/README.md`](docs/tour/README.md) — an introductory guide for
  first-time users. Read top to bottom
- [`docs/usage/execsandbox.md`](docs/usage/execsandbox.md) — the list of
  launch options for the generated executable
- [`docs/usage/execsandbox-build.md`](docs/usage/execsandbox-build.md) —
  how to use the builder
- [`docs/examples/README.md`](docs/examples/README.md) — examples, with
  working code and actual output
- [`docs/spec/execsandbox_spec.md`](docs/spec/execsandbox_spec.md) — the
  specification (includes the rationale behind design decisions)

## License

[MIT](LICENSE). The generated executable also contains
[wazero](https://github.com/tetratelabs/wazero)'s code (Apache-2.0), so
distributing it to a third party carries both licenses' attribution
obligations. The output itself can print the required text via `-L,
--print-licenses`.

---

日本語版は [README_ja.md](README_ja.md) を参照してください。
