package sandbox

// ゲストへ与えるサンドボックスポリシー(仕様書§8)を、wazeroの設定
// （ModuleConfig）へ翻訳する。CLIオプションの書式解釈（"KEY=VALUE"の分割、
// カンマ区切りの集合など）はcmd側の責務であり、ここでは解釈済みの値だけを
// 受け取る（.claude/rules/directory-structure.md）。

import (
	"crypto/rand"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero"
)

// EnvVar はゲストへ渡す環境変数1件（-e/--env）。
type EnvVar struct {
	Key, Value string
}

// Stdio はどの標準入出力をシェルへ接続するか（-s/--stdio）。falseのままの
// ストリームはwazeroの既定（Stdinはio.EOF、Stdout/Stderrはio.Discard）に
// 委ね、明示的に有効化しない限り何もできないという原則を保つ
// （.claude/rules/cli-output.md）。
type Stdio struct {
	In, Out, Err bool
}

// Mount はゲストへ見せるディレクトリマウント1件（-v/--volume）。
type Mount struct {
	Host, Guest string
	ReadOnly    bool
}

// Deny はどのケイパビリティを明示的に遮断するか（-x/--deny）。仕様書§8.2で
// 乱数・時刻は既定で許可としているが、wazero自身の既定はこれと逆
// （決定的な乱数、偽の単調時計）である点に注意。falseのままなら
// ModuleConfigで明示的に有効化し、仕様の既定（許可）を成立させる
// （.claude/rules/wazero-quirks.md、Step 8で新設予定）。
type Deny struct {
	Random, Time bool
}

// Policy は起動時オプションから組み立てたポリシー一式。
type Policy struct {
	Env   []EnvVar
	Stdio Stdio
	// Args は"--"以降のゲスト引数。argv[0]は含めない
	// （ModuleConfigが補う。確認済み方針）。
	Args   []string
	Mounts []Mount
	Deny   Deny
	// MemoryLimitBytes はWASM線形メモリの上限（-m/--mem-limit）。バイト単位。
	MemoryLimitBytes int64

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// ModuleConfig はPolicyをwazero.ModuleConfigへ翻訳する。
func (p Policy) ModuleConfig() wazero.ModuleConfig {
	cfg := wazero.NewModuleConfig().WithArgs(append([]string{"execsandbox"}, p.Args...)...)

	for _, e := range p.Env {
		cfg = cfg.WithEnv(e.Key, e.Value)
	}

	if p.Stdio.In {
		cfg = cfg.WithStdin(p.Stdin)
	}
	if p.Stdio.Out {
		cfg = cfg.WithStdout(p.Stdout)
	}
	if p.Stdio.Err {
		cfg = cfg.WithStderr(p.Stderr)
	}

	// マウントが空なら呼ばない。wazeroの既定（FSConfigなし）は
	// path_open等がsyscall.ENOSYSを返す、つまりファイルシステムへの
	// アクセスが一切できない状態であり、それが-v未指定時の既定であるべき
	// ため（.claude/rules/cli-output.md）。
	if len(p.Mounts) > 0 {
		fsConfig := wazero.NewFSConfig()
		for _, m := range p.Mounts {
			if m.ReadOnly {
				fsConfig = fsConfig.WithReadOnlyDirMount(m.Host, m.Guest)
			} else {
				fsConfig = fsConfig.WithDirMount(m.Host, m.Guest)
			}
		}
		cfg = cfg.WithFSConfig(fsConfig)
	}

	// 乱数: 仕様書§8.2は既定で許可。wazeroの既定は決定的な乱数（毎回同じ
	// バイト列）であり、これは「許可」ではなく「予測可能」という別の危険を
	// 生むため、-x randomがない限り明示的にcrypto/rand.Readerへ切り替える。
	// -x randomの場合は常にエラーを返すreaderを渡す。wazeroの決定的乱数を
	// そのまま使わないのは、「遮断」のつもりが「予測可能な乱数を許可」に
	// すり替わってしまうのを避けるため。
	if p.Deny.Random {
		cfg = cfg.WithRandSource(alwaysErrorReader{})
	} else {
		cfg = cfg.WithRandSource(rand.Reader)
	}

	// 時刻: 仕様書§8.2は既定で許可。wazeroの既定は偽の単調時計（1回の
	// 呼び出しごとに1ms進むだけ）であり、これも「許可」ではなく「嘘の時刻」
	// になるため、-x timeがない限り明示的に実時刻へ切り替える。
	// nanotimeだけ有効化するとGoランタイムのsleep実装がビジーループに
	// 陥るため、walltime/nanotime/nanosleepは必ず三点セットで扱う。
	// -x timeの場合は何もしない。WASIのclock_time_getにエラー経路がなく
	// 「取得を遮断」を表現できないため、wazeroの既定（偽の単調時計）の
	// ままにするのが実効的な「時刻を見せない」手段になる
	// （docs/usageに明記する、Step 8）。
	if !p.Deny.Time {
		cfg = cfg.WithSysWalltime().WithSysNanotime().WithSysNanosleep()
	}

	return cfg
}

// alwaysErrorReader は-x randomの実装。wazeroの決定的乱数（毎回同じ結果に
// なるだけで「乱数が取れてしまう」ことに変わりはない）を流用せず、常に
// 読み取りエラーにすることでrandom_getをEIOにする（確認済み方針）。
type alwaysErrorReader struct{}

func (alwaysErrorReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("random is denied by -x/--deny")
}

// wasmPageSize はWASM線形メモリの1ページのバイト数（固定、仕様で規定）。
const wasmPageSize = 1 << 16

// maxMemoryLimitPages はwazeroの既定上限（4GiB）。WithMemoryLimitPagesに
// これを超える値を渡すとpanicするため、変換の時点で弾く
// （.claude/rules/testing.mdのCI観点ではなく、wazero自体の制約）。
const maxMemoryLimitPages = 65536

// MemoryLimitPages はバイト数をwazeroのページ数（64KiB単位）へ変換する。
// 端数はページ単位でしか制限を表現できないため切り上げる（例えば-m 1Kは
// 64KiBに丸められる）。切り上げにより指定値未満の上限になることを避ける
// ため（切り下げると0ページになりうる入力があり、モジュールの最小メモリ
// 確保にすら満たず即座に失敗する）。
func MemoryLimitPages(bytes int64) (uint32, error) {
	if bytes <= 0 {
		return 0, fmt.Errorf("memory limit must be greater than 0 bytes")
	}
	pages := (bytes + wasmPageSize - 1) / wasmPageSize
	if pages > maxMemoryLimitPages {
		return 0, fmt.Errorf("memory limit %d bytes exceeds the maximum of %d bytes (%d pages of %d bytes)",
			bytes, int64(maxMemoryLimitPages)*wasmPageSize, maxMemoryLimitPages, wasmPageSize)
	}
	return uint32(pages), nil
}

// RuntimeConfig はPolicyのメモリ上限をwazero.RuntimeConfigへ翻訳する。
// ModuleConfigと異なりRuntime生成時（wazero.NewRuntimeWithConfig）にしか
// 渡せないため、メソッドを分けている。
//
// WithMemoryCapacityFromMaxは呼ばない。上限とは「超えてはいけない天井」で
// あって「あらかじめ確保しておく量」ではないため（仕様書§8.1）。呼んでしまうと
// -m 512M（既定）だけで毎回512MiBを先行確保することになり、軽量なゲストほど
// 無駄が大きい。
func (p Policy) RuntimeConfig() (wazero.RuntimeConfig, error) {
	pages, err := MemoryLimitPages(p.MemoryLimitBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid -m/--mem-limit: %w", err)
	}
	return wazero.NewRuntimeConfig().WithMemoryLimitPages(pages), nil
}
