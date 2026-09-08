// cmd/execsandbox は、ビルダーが埋め込むベースバイナリ本体。
//
// フェーズ①Step 2時点では、自身の末尾からスタンプされたWASMモジュールを
// 取り出せることの確認のみを行う。wazeroによる実行（ホスト関数の登録・
// メールボックス連携）はフェーズ①Step 3で追加する。
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/amisonnet8/execsandbox/sandbox"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "execsandbox: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate own executable: %w", err)
	}

	f, err := os.Open(selfPath)
	if err != nil {
		return fmt.Errorf("open own executable: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat own executable: %w", err)
	}

	wasm, err := sandbox.ExtractWASM(f, stat.Size())
	if err != nil {
		if errors.Is(err, sandbox.ErrNotStamped) {
			return errors.New("no WASM module embedded in this binary; stamp one with the execsandbox-build tool first")
		}
		return err
	}

	// フェーズ①Step 3でwazero実行に置き換える。現時点では自己読み出しの確認のみ。
	fmt.Fprintf(os.Stderr, "execsandbox: found embedded WASM module (%d bytes)\n", len(wasm))
	return nil
}
