// deny demonstrates -x/--deny: it prints one random byte and the current
// time as seen by the guest. Compare the output with and without
// -x random,time to see the effect.
//
// Build with TinyGo for the wasip1 target:
//
//	tinygo build -target=wasip1 -o deny.wasm .
package main

import (
	"crypto/rand"
	"fmt"
	"time"
)

func main() {
	buf := make([]byte, 1)
	if _, err := rand.Read(buf); err != nil {
		fmt.Printf("random_get failed: %v\n", err)
	} else {
		fmt.Printf("random byte: %d\n", buf[0])
	}
	fmt.Printf("clock: %s\n", time.Now().UTC().Format(time.RFC3339))
}
