// cmd/execsandbox-build は、`.wasm` モジュールをExecSandbox本体（ベース
// バイナリ）に埋め込み、単一の実行ファイルを生成するビルダー（仕様書§6）。
//
// フェーズ④Step1時点では、CLIのパース・検証と、指定されたターゲットの
// ベースバイナリ・入力wasmが読み込めることの確認までを実装している。実際の
// スタンプ（フッター書き込み）はStep2で実装する
// （フェーズ①Step2〜3と同じ「まず読み込みを確認し、後段の処理を差し替える」
// 進め方）。
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

	// フェーズ④Step1時点ではスタンプ（フッター書き込み）が未実装のため、
	// 読み込めたバイト数を報告するだけ（Step2で置き換える）。
	fmt.Fprintf(os.Stderr, "execsandbox-build: loaded base binary for %s (%d bytes) and %s (%d bytes)\n",
		opts.target, len(base), opts.wasmPath, len(wasm))
	return nil
}
