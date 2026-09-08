// tests/stamp は、E2Eテストで使う「ベースバイナリにWASMモジュールをスタンプする」
// ための最小限のツール。実際のビルダー（cmd/execsandbox-build、フェーズ④）の
// 代用であり、配布物ではない（PLAN.md フェーズ①Step2の記録を参照——「E2Eで
// 恒常的なスタンプ手段が必要になった時点で改めて用意する」としていたもの）。
//
// フッターの書き込みロジックをsandboxパッケージと共有しないのは、
// .claude/rules/directory-structure.mdが定める「フッターの書き込みは
// ビルダーの責務であり、読み出し側（sandbox.ExtractWASM）とは独立に実装する」
// という切り分けに倣うため。
package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	footerMagic   = "EXECSB01"
	footerSize    = 32
	footerVersion = uint32(1)
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: stamp <base> <wasm> <out>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, "stamp:", err)
		os.Exit(1)
	}
}

func run(basePath, wasmPath, outPath string) error {
	base, err := os.ReadFile(basePath)
	if err != nil {
		return err
	}
	wasm, err := os.ReadFile(wasmPath)
	if err != nil {
		return err
	}

	footer := make([]byte, footerSize)
	copy(footer[0:8], footerMagic)
	binary.BigEndian.PutUint32(footer[8:12], footerVersion)
	binary.BigEndian.PutUint64(footer[12:20], uint64(len(base)))
	binary.BigEndian.PutUint64(footer[20:28], uint64(len(wasm)))
	// footer[28:32] はReserved（ゼロのまま）。

	out := make([]byte, 0, len(base)+len(wasm)+footerSize)
	out = append(out, base...)
	out = append(out, wasm...)
	out = append(out, footer...)

	return os.WriteFile(outPath, out, 0o755)
}
