日本語版: [03-nothing-by-default_ja.md](03-nothing-by-default_ja.md)

# 3. Nothing by Default

ExecSandbox's capabilities are **denied by default**. Unless you make it
explicit through a launch-time option, the guest sees nothing — not
standard output, not files, not the network, not environment variables.
Let's confirm this with code.

Below is a minimal guest that uses nothing but `fmt` and `os` — it doesn't
even use the SDK
([`docs/examples/src/hello-wasi/main.go`](../examples/src/hello-wasi/main.go)).

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

Build it with TinyGo and embed it.

```
$ tinygo build -target=wasip1 -o hello-wasi.wasm .
$ execsandbox-build -o hello-wasi hello-wasi.wasm
```

Run it with no options at all, and nothing gets printed.

```
$ ./hello-wasi
$ echo $?
0
```

Even though `fmt.Println` was called three times, nothing came out. The
guest didn't crash either (exit code 0) — **standard output simply isn't
connected to anything**, so the output was silently discarded. We'll wire
it up in the next chapter.

---
[← Previous: 2. Installing and Your First Run](02-install-and-first-run.md) | [Index](README.md) | [Next: 4. Seeing Output →](04-seeing-output.md)
