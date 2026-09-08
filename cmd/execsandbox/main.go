// cmd/execsandbox は、ビルダーが埋め込むベースバイナリ本体。
//
// フェーズ①Step 4時点では、サンドボックス間通信に最低限必要な -n/--name と
// -d/--dest のみをパースする。仕様書§7.1の残りのオプション（-e/-v/-m/-b/-f/
// -l/-s/-t/-x/-q/-h/-V）と、それに応じたエラー表示の統一は「本格的な作り込み」
// であるフェーズ②で行う。そのため、ここでのフラグ解析エラーはGoの標準
// `flag`パッケージ自身の出力（execsandbox:接頭辞なし）のまま許容している。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tetratelabs/wazero"

	"github.com/amisonnet8/execsandbox/sandbox"
)

// フェーズ①Step 4時点ではCLIオプション未実装のため、仕様書§7.1の既定値を
// そのまま使う。フェーズ②で-b/--mailbox-limit、-f/--max-frameの解析結果に
// 置き換える。
const (
	defaultMailboxLimit = 1024
	defaultMaxFrame     = 1 << 20 // 1M
)

// destAssignments は "-d, --dest N=ID" を繰り返し指定できるようにする
// flag.Value実装（仕様書§3.3）。
type destAssignments map[uint32]string

func (d destAssignments) String() string {
	return "" // flagパッケージの既定値表示用。複数指定できるため特に意味を持たない。
}

func (d destAssignments) Set(s string) error {
	n, id, ok := strings.Cut(s, "=")
	if !ok || n == "" || id == "" {
		return fmt.Errorf("invalid -d/--dest value %q, want N=ID", s)
	}
	num, err := strconv.ParseUint(n, 10, 32)
	if err != nil || num == 0 {
		return fmt.Errorf("invalid destination number %q in %q, want a positive integer", n, s)
	}
	d[uint32(num)] = id
	return nil
}

func main() {
	fs := flag.NewFlagSet("execsandbox", flag.ContinueOnError)

	var name string
	fs.StringVar(&name, "n", "", "own ID for sandbox-to-sandbox messaging (see --name)")
	fs.StringVar(&name, "name", "", "own ID for sandbox-to-sandbox messaging; without it, this instance does not receive sandbox-to-sandbox messages")

	dest := make(destAssignments)
	fs.Var(dest, "d", "assign a destination number to an ID, N=ID (see --dest)")
	fs.Var(dest, "dest", "assign a destination number to an ID, N=ID; repeatable")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2) // flagパッケージが既にUsage/エラーメッセージを出力済み
	}

	if err := run(name, dest); err != nil {
		fmt.Fprintf(os.Stderr, "execsandbox: %s\n", err)
		os.Exit(1)
	}
}

func run(name string, dest destAssignments) error {
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

	mailbox := sandbox.NewMailbox(defaultMailboxLimit, os.Stderr)

	if name != "" {
		listener, err := sandbox.Listen(name)
		if err != nil {
			return fmt.Errorf("listen for sandbox-to-sandbox messages: %w", err)
		}
		defer listener.Close()
		go sandbox.Serve(listener, mailbox, defaultMaxFrame, os.Stderr)
	}

	destTable := sandbox.NewDestTable(dest)
	defer destTable.Close()

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	if _, err := sandbox.RegisterHostModule(ctx, rt, sandbox.HostConfig{
		Mailbox:  mailbox,
		MaxFrame: defaultMaxFrame,
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
