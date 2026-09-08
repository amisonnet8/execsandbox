package sandbox

// ゲストへ与えるサンドボックスポリシー(仕様書§8)を、wazeroの設定
// （ModuleConfig）へ翻訳する。CLIオプションの書式解釈（"KEY=VALUE"の分割、
// カンマ区切りの集合など）はcmd側の責務であり、ここでは解釈済みの値だけを
// 受け取る（.claude/rules/directory-structure.md）。

import (
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

// Policy は起動時オプションから組み立てたポリシー一式。
type Policy struct {
	Env   []EnvVar
	Stdio Stdio
	// Args は"--"以降のゲスト引数。argv[0]は含めない
	// （ModuleConfigが補う。確認済み方針）。
	Args []string

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

	return cfg
}
