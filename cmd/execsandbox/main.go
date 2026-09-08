// cmd/execsandbox は、ビルダーが埋め込むベースバイナリ本体。
//
// フェーズ②Step6時点で、仕様書§7.1の全オプションのパース・検証・ヘルプ・
// バージョン表示に加え、-q/--quietによるホスト側ログの抑制、WASI組み込みと
// -e/-s/"--"以降の引数・-v（ファイルシステム）・-m（メモリ上限）・
// -x（乱数・時刻）の配線を実装した。-tの値はまだwazeroへ配線していない
// （タイムアウトはフェーズ②の以降のステップで行う）。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

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

	log := sandbox.NewLogger(os.Stderr, opts.quiet)

	mailbox := sandbox.NewMailbox(opts.mailboxLimit, log)

	if opts.name != "" {
		listener, err := sandbox.Listen(opts.name)
		if err != nil {
			return fmt.Errorf("listen for sandbox-to-sandbox messages: %w", err)
		}
		defer listener.Close()
		go sandbox.Serve(listener, mailbox, int(opts.maxFrame), log)
	}

	destTable := sandbox.NewDestTable(opts.dest)
	defer destTable.Close()

	policy := sandbox.Policy{
		Env:              toSandboxEnv(opts.env),
		Stdio:            sandbox.Stdio(opts.stdio),
		Args:             opts.guestArgs,
		Mounts:           toSandboxMounts(opts.volumes),
		Deny:             sandbox.Deny(opts.deny),
		MemoryLimitBytes: opts.memLimit,
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
	}

	rtConfig, err := policy.RuntimeConfig()
	if err != nil {
		return err
	}

	ctx := context.Background()
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return fmt.Errorf("register WASI module: %w", err)
	}

	if _, err := sandbox.RegisterHostModule(ctx, rt, sandbox.HostConfig{
		Mailbox:  mailbox,
		MaxFrame: int(opts.maxFrame),
		Log:      log,
		Dest:     destTable,
	}); err != nil {
		return fmt.Errorf("register host module: %w", err)
	}

	// wazeroの既定StartFunctions（"_start"）により、ゲストのエントリポイントが
	// 自動実行される。ゲストは通常ここでrecv()のループへ入り常駐する。
	if _, err := rt.InstantiateWithConfig(ctx, wasmBytes, policy.ModuleConfig()); err != nil {
		return fmt.Errorf("run WASM module: %w", err)
	}
	return nil
}

func toSandboxEnv(env []envVar) []sandbox.EnvVar {
	out := make([]sandbox.EnvVar, len(env))
	for i, e := range env {
		out[i] = sandbox.EnvVar{Key: e.Key, Value: e.Value}
	}
	return out
}

func toSandboxMounts(volumes []volumeMount) []sandbox.Mount {
	out := make([]sandbox.Mount, len(volumes))
	for i, v := range volumes {
		out[i] = sandbox.Mount{Host: v.Host, Guest: v.Guest, ReadOnly: v.ReadOnly}
	}
	return out
}
