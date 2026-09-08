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
	"github.com/tetratelabs/wazero/api"
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

// callWriteProbe はwasi_probe.wasmのwrite_probe(path,data)を呼ぶ。
// path/dataはゲストの線形メモリの空き領域(20000番地以降)へ書き込んでから
// 呼び出す。
func callWriteProbe(t *testing.T, ctx context.Context, mod api.Module, path, data string) int32 {
	t.Helper()
	const pathOff, dataOff = 20000, 20100

	if !mod.Memory().Write(pathOff, []byte(path)) {
		t.Fatalf("write path into guest memory")
	}
	if !mod.Memory().Write(dataOff, []byte(data)) {
		t.Fatalf("write data into guest memory")
	}

	res, err := mod.ExportedFunction("write_probe").Call(ctx,
		uint64(pathOff), uint64(len(path)), uint64(dataOff), uint64(len(data)))
	if err != nil {
		t.Fatalf("call write_probe: %v", err)
	}
	return int32(res[0])
}

func TestPolicy_mount_none_pathOpenFails(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	p := Policy{} // -v未指定：マウントなし
	cfg := p.ModuleConfig().WithStartFunctions()
	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, cfg)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	if errno := callWriteProbe(t, ctx, mod, "test.txt", "hello"); errno == 0 {
		t.Error("write_probe() = 0, want a path_open error (no mount configured, fd 3 must not exist)")
	}
}

func TestPolicy_mount_readWrite(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	dir := t.TempDir()
	p := Policy{Mounts: []Mount{{Host: dir, Guest: "/data"}}}
	cfg := p.ModuleConfig().WithStartFunctions()
	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, cfg)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	if errno := callWriteProbe(t, ctx, mod, "test.txt", "hello"); errno != 0 {
		t.Fatalf("write_probe() = %d, want 0 (rw mount must allow writes)", errno)
	}

	got, err := os.ReadFile(dir + "/test.txt")
	if err != nil {
		t.Fatalf("read back the file the guest wrote: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("file content = %q, want %q", got, "hello")
	}
}

func TestPolicy_mount_readOnlyRejectsWrites(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	dir := t.TempDir()
	p := Policy{Mounts: []Mount{{Host: dir, Guest: "/data", ReadOnly: true}}}
	cfg := p.ModuleConfig().WithStartFunctions()
	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, cfg)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	if errno := callWriteProbe(t, ctx, mod, "test.txt", "hello"); errno == 0 {
		t.Error("write_probe() = 0, want an error (read-only mount must reject writes)")
	}
	if _, err := os.Stat(dir + "/test.txt"); err == nil {
		t.Error("file was created on a read-only mount")
	}
}

func TestMemoryLimitPages(t *testing.T) {
	tests := []struct {
		name    string
		bytes   int64
		want    uint32
		wantErr bool
	}{
		{name: "exact page multiple", bytes: 512 * 1024 * 1024, want: 512 * 1024 * 1024 / wasmPageSize},
		{name: "rounds up a partial page", bytes: 1, want: 1},
		{name: "rounds up 1K", bytes: 1024, want: 1},
		{name: "at the maximum", bytes: int64(maxMemoryLimitPages) * wasmPageSize, want: maxMemoryLimitPages},
		{name: "zero is rejected", bytes: 0, wantErr: true},
		{name: "negative is rejected", bytes: -1, wantErr: true},
		{name: "one byte over the maximum is rejected", bytes: int64(maxMemoryLimitPages)*wasmPageSize + 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MemoryLimitPages(tt.bytes)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("MemoryLimitPages(%d) = %d, want error", tt.bytes, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("MemoryLimitPages(%d) error = %v", tt.bytes, err)
			}
			if got != tt.want {
				t.Errorf("MemoryLimitPages(%d) = %d, want %d", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestPolicy_runtimeConfig_enforcesMemoryLimit(t *testing.T) {
	ctx := context.Background()

	wasmBytes, err := os.ReadFile("../testdata/modules/mem_hog.wasm")
	if err != nil {
		t.Fatalf("read mem_hog.wasm: %v", err)
	}

	// 2ページ(128KiB)に制限する。1ページ目はモジュール宣言の初期メモリで
	// 既に確保済みなので、grow_until_failは1回成功して2ページ目に到達し、
	// 2回目のgrowで失敗するはず。
	p := Policy{MemoryLimitBytes: 2 * wasmPageSize}
	rtConfig, err := p.RuntimeConfig()
	if err != nil {
		t.Fatalf("RuntimeConfig: %v", err)
	}

	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, wazero.NewModuleConfig().WithStartFunctions())
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	res, err := mod.ExportedFunction("grow_until_fail").Call(ctx)
	if err != nil {
		t.Fatalf("call grow_until_fail: %v", err)
	}
	if got, want := res[0], uint64(2); got != want {
		t.Errorf("grow_until_fail() = %d pages, want %d (the -m limit must cap growth)", got, want)
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
