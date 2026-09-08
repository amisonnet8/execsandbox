// file-access demonstrates -v/--volume: it writes a file under /data (the
// guest-side mount point) and reads it back. Run against a host launched
// with -v HOST:/data and one launched without any -v at all, to see the
// difference.
//
// Build with TinyGo for the wasip1 target:
//
//	tinygo build -target=wasip1 -o file-access.wasm .
package main

import (
	"fmt"
	"os"
)

func main() {
	const path = "/data/hello.txt"

	if err := os.WriteFile(path, []byte("written by the guest\n"), 0o644); err != nil {
		fmt.Printf("write failed: %v\n", err)
		return
	}
	fmt.Println("write ok")

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("read failed: %v\n", err)
		return
	}
	fmt.Printf("read back: %s", data)
}
