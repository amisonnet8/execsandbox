# hello-wasi — minimal setup

A guest module minimal enough to skip even the ExecSandbox SDK. Written
using nothing but WASI-standard `fmt`/`os`, it shows how `-s out`, `-e`, and
arguments after `--` reach the guest. No sandbox-to-sandbox messaging or
external connections.

Source: [`src/hello-wasi/main.go`](src/hello-wasi/main.go)

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello from execsandbox")
	fmt.Printf("args: %v\n", os.Args)
	fmt.Printf("GREETING=%s\n", os.Getenv("GREETING"))
}
```

## Build

Build for the `wasip1` target with TinyGo.

```
$ tinygo build -target=wasip1 -o hello-wasi.wasm .
```

Embed it into a single executable with `execsandbox-build`.

```
$ execsandbox-build -o hello-wasi hello-wasi.wasm
execsandbox-build: wrote hello-wasi (linux/amd64, 8788514 bytes)
```

## Run

### Without `-s out`

By default the guest's standard output isn't connected to anything (see
"everything is blocked by default" in `docs/usage/execsandbox.md`). The
guest itself runs and exits normally, but nothing is visible.

```
$ ./hello-wasi -e GREETING=konnichiwa -- foo bar
$ echo $?
0
```

### With `-s out`

```
$ ./hello-wasi -s out -e GREETING=konnichiwa -- foo bar
hello from execsandbox
args: [execsandbox foo bar]
GREETING=konnichiwa
```

## Discussion

- **`-s out`** connects the guest's standard output to the host's. Unless
  given, `fmt.Println` is silently discarded — the same "deny by default"
  philosophy from `.claude/rules/cli-output.md` is consistent in `-s` too.
- **Arguments after `--`** are passed straight through to the guest's
  `os.Args[1:]`. `os.Args[0]` isn't the guest's own file name, but the fixed
  string `"execsandbox"` (see `docs/usage/execsandbox.md`) — the embedded
  WASM module has no notion of a file name to begin with.
- **`-e KEY=VALUE`** shows the guest only the environment variables you
  specify. The host's own shell environment is never inherited as-is.

---

日本語版: [hello-wasi_ja.md](hello-wasi_ja.md)
