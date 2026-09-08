// mem-limit demonstrates -m/--mem-limit: it grows a slice one megabyte at
// a time, keeping every chunk alive so the guest's actual memory usage
// grows too (not just a discardable allocation). Once linear memory hits
// the ceiling set by -m, TinyGo's allocator reports an out-of-memory
// fatal error and the guest exits — the host process itself is
// unaffected.
//
// Build with TinyGo for the wasip1 target:
//
//	tinygo build -target=wasip1 -o mem-limit.wasm .
package main

import "fmt"

func main() {
	const step = 1 << 20 // 1 MiB
	var chunks [][]byte

	for i := 0; i < 4096; i++ { // 4096 * 1 MiB = 4 GiB safety cap
		chunks = append(chunks, make([]byte, step))
		fmt.Printf("allocated %d MiB so far\n", len(chunks))
	}
}
