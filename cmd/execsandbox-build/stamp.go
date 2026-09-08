package main

// フッターの書き込み(仕様書§6.2)。
//
// 読み出し側(sandbox.ExtractWASM、sandbox/footer.go)とは独立した実装と
// する(.claude/rules/directory-structure.md「フッターの書き込みは
// ビルダーの責務」、tests/stampと同じ切り分け)。定数(Magic・サイズ・
// レイアウト)は読み出し側と一致させること。

import (
	"encoding/binary"
	"strings"
)

const (
	// footerMagic はsandbox/footer.goのfooterMagicと一致させること。
	footerMagic   = "EXECSB01"
	footerSize    = 32
	footerVersion = uint32(1)
)

// stamp はbaseの末尾にwasmとフッターを追記したバイト列を返す。
// base・wasmは変更しない(呼び出し側のembedデータ・入力ファイルを
// 書き換えないため。.claude/rules/stamp.md「ベースバイナリは書き換えず、
// コピーに追記する」)。
func stamp(base, wasm []byte) []byte {
	footer := make([]byte, footerSize)
	copy(footer[0:8], footerMagic)
	binary.BigEndian.PutUint32(footer[8:12], footerVersion)
	binary.BigEndian.PutUint64(footer[12:20], uint64(len(base)))
	binary.BigEndian.PutUint64(footer[20:28], uint64(len(wasm)))
	// footer[28:32] はReserved（ゼロ埋めのまま）。

	out := make([]byte, 0, len(base)+len(wasm)+footerSize)
	out = append(out, base...)
	out = append(out, wasm...)
	out = append(out, footer...)
	return out
}

// finalOutputPath はopts.outputに、targetがwindowsかつ拡張子が".exe"で
// 終わっていない場合のみ".exe"を補う(仕様書§6.1「windows/*を指定した
// 場合、出力ファイル名に.exeが付く」)。
func finalOutputPath(opts *options) string {
	name := opts.output
	if opts.target.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	return name
}
