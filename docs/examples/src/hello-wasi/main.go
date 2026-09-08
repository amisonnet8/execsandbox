// hello-wasi is the minimal ExecSandbox guest: it uses nothing but WASI
// standard I/O (no ExecSandbox SDK, no sandbox-to-sandbox messaging). It
// prints its own arguments and one environment variable, to show how
// -s out / -e / "--" reach the guest.
//
// Build with TinyGo for the wasip1 target:
//
//	tinygo build -target=wasip1 -o hello-wasi.wasm .
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
