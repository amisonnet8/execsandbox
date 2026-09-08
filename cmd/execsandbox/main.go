// cmd/execsandbox は、ビルダーが埋め込むベースバイナリ本体。
//
// フェーズ①Step 3時点では、埋め込まれたWASMモジュールをwazeroで実行し、
// send/recv/max_frameのみを配線する。外部接続（conn_write）はフェーズ③、
// サンドボックス間のAF_UNIX接続（宛先解決・実際の配送）はフェーズ①Step 4、
// CLIオプション（-n/-d/-b/-f等）による設定はフェーズ②で追加する。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"

	"github.com/amisonnet8/execsandbox/sandbox"
)

// フェーズ①Step 3時点ではCLIオプション未実装のため、仕様書§7.1の既定値を
// そのまま使う。フェーズ②で-b/--mailbox-limit、-f/--max-frameの解析結果に
// 置き換える。
const (
	defaultMailboxLimit = 1024
	defaultMaxFrame     = 1 << 20 // 1M
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

	wasmBytes, err := sandbox.ExtractWASM(f, stat.Size())
	if err != nil {
		if errors.Is(err, sandbox.ErrNotStamped) {
			return errors.New("no WASM module embedded in this binary; stamp one with the execsandbox-build tool first")
		}
		return err
	}

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	mailbox := sandbox.NewMailbox(defaultMailboxLimit, os.Stderr)
	if _, err := sandbox.RegisterHostModule(ctx, rt, sandbox.HostConfig{
		Mailbox:  mailbox,
		MaxFrame: defaultMaxFrame,
		Log:      os.Stderr,
	}); err != nil {
		return fmt.Errorf("register host module: %w", err)
	}

	// wazeroの既定StartFunctions（"_start"）により、ゲストのエントリポイントが
	// 自動実行される。ゲストは通常ここでrecv()のループへ入り常駐する。
	if _, err := rt.Instantiate(ctx, wasmBytes); err != nil {
		return fmt.Errorf("run WASM module: %w", err)
	}
	return nil
}
