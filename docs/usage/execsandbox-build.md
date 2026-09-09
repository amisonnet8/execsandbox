# execsandbox-build — the builder

Embeds a `.wasm` file into the ExecSandbox core, producing a single
executable file.

```
execsandbox-build -o <output file> [--target <GOOS>/<GOARCH>] <input.wasm>
```

**The contents of this page are measured values as of the completion of
Phase 4.** The actual `--help` output is as follows (`-V` prints `dev` on a
development build that predates the `-ldflags -X main.version=` embedding
done at build time).

```
$ execsandbox-build --help
Usage: execsandbox-build -o <output> [--target GOOS/GOARCH] <input.wasm>

Embeds a WASM module into ExecSandbox to produce a single self-contained
executable (spec §6). The builder does not embed any sandbox policy; all
capabilities (filesystem, network, environment, ...) are chosen at the
generated executable's startup, not at build time.

Options:
  -o, --output NAME     output file name (required)
      --target GOOS/GOARCH
                         target platform (default: this machine's platform)
  -h, --help             show this help message
  -V, --version          print the version

Supported targets:
  linux/amd64     linux/arm64
  darwin/amd64    darwin/arm64
  windows/amd64   windows/arm64

If --target selects windows, ".exe" is appended to the output file name
when it isn't already present.

The generated executable embeds ExecSandbox (MIT) and wazero (Apache-2.0).
Distributing it to a third party carries their attribution obligations.
```

## Installation

**Download the builder matching your environment from GitHub Releases.**
Assets named `execsandbox-build_<tag>_<GOOS>_<GOARCH>` (`.exe` for Windows
only) are published for six platforms. Just download it and set the
executable bit — no Go toolchain required.

```
curl -LO https://github.com/amisonnet8/execsandbox/releases/download/<tag>/execsandbox-build_<tag>_linux_amd64
chmod +x execsandbox-build_<tag>_linux_amd64
```

Each asset ships with a `.sha256` file, so you can verify it after
downloading.

```
sha256sum -c execsandbox-build_<tag>_linux_amd64.sha256
```

**`go install` is not supported.** Per spec §6.3, the builder is designed to
embed six platforms' worth of base binaries into itself via `//go:embed`,
but that embed target is not committed to the repository (following the
policy of not committing distributables to the repository,
`.claude/rules/distribution.md`). As a result, running `go install
.../cmd/execsandbox-build@latest` would produce a builder whose embed is
empty — meaning no target would find its base binary, and it would error
regardless of which target is chosen. The six binaries on GitHub Releases
are the only distribution channel.

## Options

| Short | Long | Argument | Description | Default |
| :--- | :--- | :--- | :--- | :--- |
| `-o` | `--output` | file name | Output executable's name | (required) |
| — | `--target` | `GOOS/GOARCH` | Target platform | the builder's own platform |
| `-h` | `--help` | — | Help | — |
| `-V` | `--version` | — | Version | — |

`--target` has no short form because it's used far less often than the
other options (`-o`/`-h`/`-V`), and because the `GOOS/GOARCH` form itself
carries enough information that it resists abbreviation.

## Usage

```
# For the same platform you're running on
execsandbox-build -o mydb mydb.wasm

# Cross-build
execsandbox-build -o mydb --target linux/arm64 mydb.wasm
```

Because the builder embeds base binaries for every supported platform, you
can cross-build just by changing `--target`. No Go toolchain required.

## Supported Platforms

```
linux/amd64     linux/arm64
darwin/amd64    darwin/arm64
windows/amd64   windows/arm64
```

When `windows/*` is specified, `.exe` (case-insensitive) is automatically
appended if the output file name doesn't already end with it.

Specifying a value for `--target` outside these six is an error, and
nothing is produced (see "Startup option errors and exit codes" below).

## What Gets Produced

```
[base binary][WASM module][footer]
```

The base binary contains the ExecSandbox core and `wazero`. At startup, the
generated executable reads the WASM module from its own tail and runs it.

The builder never bakes in any sandbox policy whatsoever. Mounts, memory
limits, listen addresses, and everything else are all specified at
**launch time** ([`execsandbox.md`](execsandbox.md)). The same WASM module
can be turned into an executable, then launched any number of times with
different wiring or different limits.

The generated executable gets its executable bit set (`0o755`). Windows has
no concept of an executable bit, so this setting has no effect there (it's
ignored).

## About License Notices

The generated executable contains code from both the ExecSandbox core (MIT)
and `wazero` (Apache-2.0). **Distributing the output to a third party
carries an obligation to include both parties' notices.**

| Subject | License | Required action |
| :--- | :--- | :--- |
| ExecSandbox | MIT | Include the copyright notice and full license text |
| wazero | Apache-2.0 | Include the copyright notice, full license text, and NOTICE |

**This obligation can be satisfied by the generated executable itself,
rather than by the builder.** Passing `-L, --print-licenses` to the output
prints all the required text to standard output (see "`-L,
--print-licenses`" in [`execsandbox.md`](execsandbox.md)).

```
./mydb -L > THIRD-PARTY-LICENSES.txt
```

Since it's the output, not the builder, that gets distributed to third
parties, this is arranged so that the output itself can satisfy the
obligation with this command even if the distributor has no access to the
builder.

The WASM module's own license is entirely free to choose. Since neither
license is copyleft, the output as a whole can be distributed under any
license, including a proprietary one.

## About Code Signing

The generated executable ships unsigned, which isn't unusual for a
cross-platform CLI tool. macOS and Windows may show a warning on first
launch (Gatekeeper, SmartScreen).

That said, appending to the end of the file breaks any existing signature
(a signature is verified against a hash of the whole file), so signing
before stamping wouldn't help anyway. If you need a signature for
distribution, re-sign it yourself after generation.

## Startup Option Errors and Exit Codes

| Situation | Exit code | Output destination |
| :--- | :--- | :--- |
| `--help` / `-h` | 0 | standard output |
| `--version` / `-V` | 0 | standard output |
| `-o/--output` missing, the input `.wasm` is missing or given more than once, `--target`'s syntax or value is invalid | 2 | standard error (`execsandbox-build:` prefix) |
| The input `.wasm` can't be read, the output file can't be written, or another reason generation failed | 1 | standard error (`execsandbox-build:` prefix) |
| Generation succeeded | 0 | — |

An actual example of an option error (exit code 2):

```
$ execsandbox-build -o mydb
execsandbox-build: missing input .wasm file
execsandbox-build: run with --help for usage
```

---

日本語版: [execsandbox-build_ja.md](execsandbox-build_ja.md)
