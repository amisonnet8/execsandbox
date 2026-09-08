package main

// ヘルプ文言(仕様書§6.1)。cmd/execsandbox/usage.goと同じく、
// fs.PrintDefaults()は短形・長形が別エントリになり表形式にできないため
// 使わず、手書きにする。

import (
	"fmt"
	"io"
)

const usageText = `Usage: execsandbox-build -o <output> [--target GOOS/GOARCH] <input.wasm>

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
`

func writeUsage(w io.Writer) {
	fmt.Fprint(w, usageText)
}
