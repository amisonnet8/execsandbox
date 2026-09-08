// timeout demonstrates -t/--timeout: it loops forever without ever
// checking for cancellation, so the only way it stops is the host's
// forced termination when the time limit elapses.
//
// Build with TinyGo for the wasip1 target:
//
//	tinygo build -target=wasip1 -o timeout.wasm .
package main

func main() {
	for {
	}
}
