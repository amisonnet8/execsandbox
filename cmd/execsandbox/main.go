// cmd/execsandbox は、ビルダーが埋め込むベースバイナリ本体。
//
// フェーズ②Step7時点で、仕様書§7.1の全オプションのパース・検証・ヘルプ・
// バージョン表示に加え、-q/--quietによるホスト側ログの抑制、WASI組み込みと
// -e/-s/"--"以降の引数・-v（ファイルシステム）・-m（メモリ上限）・
// -x（乱数・時刻）・-t（タイムアウト）の配線を実装した。仕様書§7.1の
// 全オプションが実際に配線済み（-lは仕様書にあるがフェーズ③スコープのため
// フラグ自体を未定義のまま）。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"

	"github.com/amisonnet8/execsandbox/sandbox"
)

// version はビルド時に `-ldflags -X main.version=<tag>` で上書きする
// （フェーズ④のリリースパイプラインが担う。バージョン埋め込み方式全体の
// 整理はフェーズ④、PLAN.md「保留事項」参照）。
var version = "dev"

// timeoutExitCode は-t/--timeoutで実行時間の上限に達した場合のプロセス
// 終了コード。Unixの`timeout(1)`コマンドが同じ場面で使う124に倣う
// （仕様書は終了コード体系を規定していないため、この対応はコメントに
// 留め仕様書は変更しない。確認済み方針）。
const timeoutExitCode = 124

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

	exitCode, err := run(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "execsandbox: %s\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

// run はサンドボックスを起動・実行する。戻り値のexitCodeは、errがnilの
// 場合にプロセスの終了コードとして使う（ゲスト自身のproc_exit等による
// 終了コードをホスト側のエラーとして扱わずそのまま伝えるため。仕様書は
// 終了コード体系を規定していないため、対応関係は実装コメントに留める。
// 確認済み方針）。errが非nilの場合、exitCodeの値は無視され、呼び出し側が
// execsandbox:接頭辞を付けて出力したうえで固定の終了コード(1)を使う。
func run(opts *options) (exitCode int, err error) {
	selfPath, err := os.Executable()
	if err != nil {
		return 1, fmt.Errorf("locate own executable: %w", err)
	}

	f, err := os.Open(selfPath)
	if err != nil {
		return 1, fmt.Errorf("open own executable: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return 1, fmt.Errorf("stat own executable: %w", err)
	}

	wasmBytes, err := sandbox.ExtractWASM(f, stat.Size())
	if err != nil {
		if errors.Is(err, sandbox.ErrNotStamped) {
			return 1, errors.New("no WASM module embedded in this binary; stamp one with the execsandbox-build tool first")
		}
		return 1, err
	}

	log := sandbox.NewLogger(os.Stderr, opts.quiet)

	mailbox := sandbox.NewMailbox(opts.mailboxLimit, log)

	if opts.name != "" {
		listener, err := sandbox.Listen(opts.name)
		if err != nil {
			return 1, fmt.Errorf("listen for sandbox-to-sandbox messages: %w", err)
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
		Timeout:          opts.timeout,
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
	}

	rtConfig, err := policy.RuntimeConfig()
	if err != nil {
		return 1, err
	}

	ctx := context.Background()
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return 1, fmt.Errorf("register WASI module: %w", err)
	}

	if _, err := sandbox.RegisterHostModule(ctx, rt, sandbox.HostConfig{
		Mailbox:  mailbox,
		MaxFrame: int(opts.maxFrame),
		Log:      log,
		Dest:     destTable,
	}); err != nil {
		return 1, fmt.Errorf("register host module: %w", err)
	}

	// -t/--timeoutが指定されている場合のみ、ゲスト実行(InstantiateWithConfig)
	// にdeadline付きのcontextを渡す。ホスト関数登録・リスナー起動は
	// context.Background()側で行っており、ここでのcontextはゲスト実行にしか
	// 影響しない。
	runCtx := ctx
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}

	// wazeroの既定StartFunctions（"_start"）により、ゲストのエントリポイントが
	// 自動実行される。ゲストは通常ここでrecv()のループへ入り常駐する。
	_, err = rt.InstantiateWithConfig(runCtx, wasmBytes, policy.ModuleConfig())
	if err == nil {
		return 0, nil
	}

	var exitErr *sys.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == sys.ExitCodeDeadlineExceeded {
			log.Printf("execution timed out after %s", opts.timeout)
			return timeoutExitCode, nil
		}
		// ゲスト自身の終了コード（proc_exit等）はホスト側のエラーではない
		// ため、execsandbox:接頭辞を付けずそのままプロセスの終了コードにする。
		return int(exitErr.ExitCode()), nil
	}
	return 1, fmt.Errorf("run WASM module: %w", err)
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
