// cmd/execsandbox-build は、`.wasm` モジュールをExecSandbox本体（ベース
// バイナリ）に埋め込み、単一の実行ファイルを生成するビルダー（仕様書§6）。
package main

import (
	"fmt"
	"os"
)

// version はビルド時に `-ldflags -X main.version=<tag>` で上書きする
// （cmd/execsandboxと同じパターン。公式リリースでは、同梱するベース
// バイナリと同じタグでビルダー自身もビルドすることで「ビルダーの
// バージョン＝生成物のバージョン」（仕様書§6.3）を保証する）。
var version = "dev"

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "execsandbox-build: %s\n", err)
		fmt.Fprintln(os.Stderr, "execsandbox-build: run with --help for usage")
		os.Exit(2)
	}

	if opts.help {
		writeUsage(os.Stdout)
		os.Exit(0)
	}
	if opts.version {
		fmt.Fprintf(os.Stdout, "execsandbox-build %s\n", version)
		os.Exit(0)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "execsandbox-build: %s\n", err)
		os.Exit(1)
	}
}

// run はビルダー本体の処理。標準出力は使わない（cmd/execsandboxのように
// ゲストの出力を通す必要はないが、将来スクリプトから利用される可能性に
// 備えて空けておく。進捗・診断情報はすべて標準エラー出力へ書く）。
func run(opts *options) error {
	base, err := loadBaseBinary(opts.target)
	if err != nil {
		return err
	}

	wasm, err := os.ReadFile(opts.wasmPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", opts.wasmPath, err)
	}

	out := stamp(base, wasm)

	outPath := finalOutputPath(opts)
	// 実行ビットを立てる(0o755)。Windowsには実行ビットの概念がなく
	// os.WriteFileのmode引数は無視されるが、無害なので分岐しない
	// (.claude/rules/stamp.md「Windowsパーミッションを検証するテストは
	// runtime.GOOS != "windows"でガードする」——ここは書き込み側であり
	// 検証ではないため、単に共通コードのままでよい)。
	if err := os.WriteFile(outPath, out, 0o755); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Fprintf(os.Stderr, "execsandbox-build: wrote %s (%s, %d bytes)\n", outPath, opts.target, len(out))
	return nil
}
