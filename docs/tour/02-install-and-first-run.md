# 2. Installing and Your First Run

## Getting the builder

Download `execsandbox-build_<tag>_<GOOS>_<GOARCH>` (`.exe` for Windows
only) matching your environment from
[Releases](https://github.com/amisonnet8/execsandbox/releases). No Go
toolchain required.

```
$ chmod +x execsandbox-build_*
```

A `.sha256` file is bundled alongside it, so it's a good idea to verify
before using it.

```
$ sha256sum -c execsandbox-build_*.sha256
```

## Embedding a WASM module

`execsandbox-build` is the "builder" that embeds a WASM module into a
single executable file. If you already have a built `.wasm` on hand, just
hand it over (how to build `mymodule.wasm` is covered starting in the next
chapter).

```
$ ./execsandbox-build -o mydb mymodule.wasm
execsandbox-build: wrote mydb (linux/amd64, 8788514 bytes)
```

## Running it

```
$ ./mydb -s out -- hello
```

`mydb` is now a standalone executable — neither `execsandbox-build` nor a
WASM runtime is needed anymore. What `-s out` and `--` are doing is
something we'll look at, one at a time, starting with the next chapter.

---
[← Previous: 1. What Is ExecSandbox?](01-what-is-execsandbox.md) | [Index](README.md) | [Next: 3. Nothing by Default →](03-nothing-by-default.md)

日本語版: [02-install-and-first-run_ja.md](02-install-and-first-run_ja.md)
