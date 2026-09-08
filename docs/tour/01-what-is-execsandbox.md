日本語版: [01-what-is-execsandbox_ja.md](01-what-is-execsandbox_ja.md)

# 1. What Is ExecSandbox?

ExecSandbox is a portable, zero-setup sandboxed execution tool that bundles
a WASM runtime and a WASM module into a single executable file. It's built
on three ideas.

- **Single-binary packaging** — everything needed to run (the runtime
  itself plus your WASM module) fits into one executable file. You never
  need anyone else to install a WASM runtime on their machine.
- **Capability-based policy** — unless you explicitly allow it at launch,
  the guest sees no files, no network, no environment variables — nothing.
- **Mailbox-style messaging** — multiple ExecSandbox instances are wired
  together through an Erlang-style mailbox to build a single system.

This tour walks you through all three, hands-on, one at a time. Each
chapter is short and builds on the one before it. Feel free to pause along
the way, or revisit only the chapters you care about.

**Prerequisite**: chapters where you build a guest module yourself use
[TinyGo](https://tinygo.org/) (and, in a couple of chapters,
[Rust](https://www.rust-lang.org/) too). How to get `execsandbox-build`
(the builder) is covered in the next chapter.

---
[Index](README.md) | [Next: 2. Installing and Your First Run →](02-install-and-first-run.md)
