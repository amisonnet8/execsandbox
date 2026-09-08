// cmd/execsandbox は、ビルダーが埋め込むベースバイナリ本体。
//
// フェーズ②Step2時点で、仕様書§7.1の全オプションのパース・検証・ヘルプ・
// バージョン表示を実装した。ただし-e/-v/-s/-t/-xの値はまだwazeroへ配線して
// いない（WASI組み込み・ファイルシステム・タイムアウト等はフェーズ②の
// 以降のステップで行う）。-n/-d/-b/-fは実際に配線済み。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"

	"github.com/amisonnet8/execsandbox/sandbox"
)

// version はビルド時に `-ldflags -X main.version=<tag>` で上書きする
// （フェーズ④のリリースパイプラインが担う。バージョン埋め込み方式全体の
// 整理はフェーズ④、PLAN.md「保留事項」参照）。
var version = "dev"

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "execsandbox: %s\n", err)
		fmt.Fprintln(os.Stderr, "execsandbox: run with --help for usage")
		os.Exit(2)
	}

	if opts.help {
		writeUsage(os.Stdout)
		os.Exit(0)
	}
	if opts.version {
		fmt.Fprintf(os.Stdout, "execsandbox %s\n", version)
		os.Exit(0)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "execsandbox: %s\n", err)
		os.Exit(1)
	}
}

func run(opts *options) error {
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

	mailbox := sandbox.NewMailbox(opts.mailboxLimit, os.Stderr)

	if opts.name != "" {
		listener, err := sandbox.Listen(opts.name)
		if err != nil {
			return fmt.Errorf("listen for sandbox-to-sandbox messages: %w", err)
		}
		defer listener.Close()
		go sandbox.Serve(listener, mailbox, int(opts.maxFrame), os.Stderr)
	}

	destTable := sandbox.NewDestTable(opts.dest)
	defer destTable.Close()

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	if _, err := sandbox.RegisterHostModule(ctx, rt, sandbox.HostConfig{
		Mailbox:  mailbox,
		MaxFrame: int(opts.maxFrame),
		Log:      os.Stderr,
		Dest:     destTable,
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
