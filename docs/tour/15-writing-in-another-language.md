# 15. Writing in Another Language

ExecSandbox's WASM ABI (spec §5) claims to be language-agnostic. We've
used only TinyGo so far — let's finally confirm this by pairing it with
Rust.

Using the same `sender`/`receiver` pair as chapter 7, we swap one side for
its Rust version.

```
$ tinygo build -target=wasip1 -o go-sender.wasm .       # TinyGo sender
$ cargo build --target wasm32-wasip1 --release --examples  # Rust receiver
$ execsandbox-build -o nodeA-go go-sender.wasm
$ execsandbox-build -o nodeB-rust receiver.wasm
```

```
$ ./nodeB-rust -n nodeB -s out &
$ ./nodeA-go -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

The reverse direction (Rust sends, TinyGo receives) works just the same.
**Without changing a single line of the receiver's code**, only the
sender's language can be swapped out. To the ExecSandbox host, both are
nothing more than a WASM module, and interoperability is achieved purely
through the ABI's contract: "the buffer is allocated on the guest side,"
"metadata is a fixed 8-byte layout."

That this holds across two languages with starkly different memory
models — TinyGo (with a GC) and Rust (no GC) — demonstrates that the ABI
doesn't depend on any particular language runtime. Writing a third-language
SDK should be able to interconnect with these two as-is, as long as it
correctly `import`s the four functions
`send`/`recv`/`conn_write`/`max_frame`.

---
[← Previous: 14. Allowing Randomness and Time](14-allowing-random-and-time.md) | [Index](README.md) | [Next: 16. Where to Go Next →](16-where-to-go-next.md)

日本語版: [15-writing-in-another-language_ja.md](15-writing-in-another-language_ja.md)
