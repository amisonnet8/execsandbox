# polyglot-messaging — interconnecting TinyGo and Rust

ExecSandbox's WASM ABI (spec §5) claims to be language-agnostic. We
confirm this in both directions, **connecting a TinyGo guest and a Rust
guest with zero code changes.** It uses the same `sender`/`receiver` pair
as [`sandbox-messaging.md`](sandbox-messaging.md), but swaps one side for
its Rust version.

Source: [`execsandbox-sdk`'s `go/examples/{sender,receiver}`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples) ·
[`rust/execsandbox/examples/{sender,receiver}.rs`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/rust/execsandbox/examples)

## Build

```
$ tinygo build -target=wasip1 -o go-sender.wasm .       # in go/examples/sender/
$ tinygo build -target=wasip1 -o go-receiver.wasm .     # in go/examples/receiver/
$ cargo build --target wasm32-wasip1 --release --examples  # in rust/execsandbox/
```

On the Rust side, `cargo build --examples` produces `target/wasm32-wasip1/
release/examples/{sender,receiver}.wasm`. Embed all four with
`execsandbox-build`.

```
$ execsandbox-build -o nodeA-go go-sender.wasm
$ execsandbox-build -o nodeB-go go-receiver.wasm
$ execsandbox-build -o nodeA-rust sender.wasm      # Rust version
$ execsandbox-build -o nodeB-rust receiver.wasm    # Rust version
```

## Run: TinyGo sends → Rust receives

```
$ ./nodeB-rust -n nodeB -s out &
$ ./nodeA-go -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

## Run: Rust sends → TinyGo receives

```
$ ./nodeB-go -n nodeB -s out &
$ ./nodeA-rust -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

## Discussion

- **Both combinations work with the exact same `nodeB` (receiver) code,
  unmodified.** The receiver neither can nor needs to tell whether the
  sender is TinyGo or Rust — to the ExecSandbox host, both are nothing more
  than a WASM module, and interoperability is achieved purely through the
  ABI's contract (`.claude/rules/abi-compatibility.md`): "the buffer is
  allocated on the guest side," "metadata is a fixed 8-byte layout."
- That this holds across two languages with starkly different memory
  models — TinyGo (with a GC) and Rust (no GC, managing buffers directly on
  the stack/heap) — demonstrates that the ABI doesn't depend on any
  particular language runtime.
- Writing a third-language SDK should be able to interconnect with these
  two as-is, as long as it correctly `import`s the four functions
  `send`/`recv`/`conn_write`/`max_frame` and follows the layout in
  [spec §5](../spec/execsandbox_spec.md).

---

日本語版: [polyglot-messaging_ja.md](polyglot-messaging_ja.md)
