package sandbox

// フェーズ②Step4: Policy.ModuleConfig()が実際にwazeroへ環境変数・引数・
// stdioを配線することを、testdata/modules/wasi_probe.wasm経由で確認する。

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func newWASIRuntime(t *testing.T) (context.Context, wazero.Runtime, []byte) {
	t.Helper()
	ctx := context.Background()

	wasmBytes, err := os.ReadFile("../testdata/modules/wasi_probe.wasm")
	if err != nil {
		t.Fatalf("read wasi_probe.wasm: %v", err)
	}

	rt := wazero.NewRuntime(ctx)
	t.Cleanup(func() { rt.Close(ctx) })

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		t.Fatalf("instantiate WASI: %v", err)
	}
	return ctx, rt, wasmBytes
}

func TestPolicy_stdoutDisabledByDefault(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	var stdout bytes.Buffer
	p := Policy{
		Args:   []string{"x", "y"},
		Stdout: &stdout,
		// Stdio.Outは既定のfalseのまま(-s outを指定しない構成)。
	}

	if _, err := rt.InstantiateWithConfig(ctx, wasmBytes, p.ModuleConfig()); err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty (stdout must stay blocked unless -s out is given)", stdout.String())
	}
}

func TestPolicy_argsAndEnvReachTheGuest(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	var stdout bytes.Buffer
	p := Policy{
		Env:    []EnvVar{{Key: "A", Value: "1"}},
		Stdio:  Stdio{Out: true},
		Args:   []string{"x", "y"},
		Stdout: &stdout,
	}

	if _, err := rt.InstantiateWithConfig(ctx, wasmBytes, p.ModuleConfig()); err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "execsandbox\x00x\x00y\x00") {
		t.Errorf("stdout = %q, want it to contain argv0 %q followed by %q and %q", got, "execsandbox", "x", "y")
	}
	if !strings.Contains(got, "A=1\x00") {
		t.Errorf("stdout = %q, want it to contain the env var %q", got, "A=1")
	}
}

func TestPolicy_stdinAndStderrGateSameWay(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	// stdinを与えず、stderrも未接続のまま_start相当を実行してもエラーには
	// ならないこと（wazeroの既定=io.EOF/io.Discardに委ねるだけ）を確認する。
	p := Policy{Args: nil}

	if _, err := rt.InstantiateWithConfig(ctx, wasmBytes, p.ModuleConfig()); err != nil {
		t.Fatalf("instantiate with no stdio enabled: %v", err)
	}
}

func TestPolicy_argcAndEnvironcProbes(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	p := Policy{
		Env:  []EnvVar{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}},
		Args: []string{"one", "two", "three"},
		// _startの自動実行はここでは不要（argc_probe/environc_probeを直接
		// 呼ぶ）ため、既定のStartFunctionsを止める。
	}
	cfg := p.ModuleConfig().WithStartFunctions()

	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, cfg)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	argc, err := mod.ExportedFunction("argc_probe").Call(ctx)
	if err != nil {
		t.Fatalf("call argc_probe: %v", err)
	}
	// argv[0]("execsandbox")を含め4個。
	if got, want := argc[0], uint64(4); got != want {
		t.Errorf("argc_probe() = %d, want %d", got, want)
	}

	environc, err := mod.ExportedFunction("environc_probe").Call(ctx)
	if err != nil {
		t.Fatalf("call environc_probe: %v", err)
	}
	if got, want := environc[0], uint64(2); got != want {
		t.Errorf("environc_probe() = %d, want %d", got, want)
	}
}
