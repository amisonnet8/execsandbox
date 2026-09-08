package sandbox

// フェーズ②Step4〜7: Policy.ModuleConfig()/RuntimeConfig()が実際にwazeroへ
// 環境変数・引数・stdio・ファイルシステム・メモリ上限・乱数・時刻・
// タイムアウトを配線することを、
// testdata/modules/{wasi_probe,mem_hog,blocker}.wasm経由で確認する。

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
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

// callRandomProbe はwasi_probe.wasmのrandom_probe(buf,len)を呼び、
// (errno, 読み取れたバイト列)を返す。
func callRandomProbe(t *testing.T, ctx context.Context, mod api.Module, n int) (int32, []byte) {
	t.Helper()
	const bufOff = 30000

	res, err := mod.ExportedFunction("random_probe").Call(ctx, uint64(bufOff), uint64(n))
	if err != nil {
		t.Fatalf("call random_probe: %v", err)
	}
	buf, ok := mod.Memory().Read(bufOff, uint32(n))
	if !ok {
		t.Fatalf("read back random buffer from guest memory")
	}
	return int32(res[0]), append([]byte(nil), buf...)
}

// callClockProbe はwasi_probe.wasmのclock_probe(id,result_ptr)を呼び、
// (errno, ナノ秒のタイムスタンプ)を返す。id=0はrealtime、1はmonotonic。
func callClockProbe(t *testing.T, ctx context.Context, mod api.Module, id uint32) (int32, uint64) {
	t.Helper()
	const resultOff = 30100

	res, err := mod.ExportedFunction("clock_probe").Call(ctx, uint64(id), uint64(resultOff))
	if err != nil {
		t.Fatalf("call clock_probe: %v", err)
	}
	ns, ok := mod.Memory().ReadUint64Le(resultOff)
	if !ok {
		t.Fatalf("read back clock timestamp from guest memory")
	}
	return int32(res[0]), ns
}

// -x未指定（既定）では乱数が本物のcrypto/rand相当であること（毎回異なる
// バイト列）を固定する。wazeroの既定は決定的な乱数であり、これを
// 見落とすと「-x random」を指定しなくても常に同じバイト列が返る
// （PLAN.md「wazeroの既定が仕様と逆転する箇所」）。
func TestPolicy_random_defaultIsNotDeterministic(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	p := Policy{}
	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, p.ModuleConfig().WithStartFunctions())
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	errno1, got1 := callRandomProbe(t, ctx, mod, 16)
	if errno1 != 0 {
		t.Fatalf("random_probe() errno = %d, want 0", errno1)
	}
	errno2, got2 := callRandomProbe(t, ctx, mod, 16)
	if errno2 != 0 {
		t.Fatalf("random_probe() errno = %d, want 0", errno2)
	}

	if bytes.Equal(got1, got2) {
		t.Errorf("two random_probe() calls returned the same bytes %x, want different (wazero's default deterministic source must not leak through)", got1)
	}
}

// -x randomは常にエラー(EIO)にする。wazeroの決定的乱数をそのまま「遮断」
// として使うと、遮断のつもりが予測可能な乱数の許可にすり替わってしまう。
func TestPolicy_random_denyReturnsError(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	p := Policy{Deny: Deny{Random: true}}
	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, p.ModuleConfig().WithStartFunctions())
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	errno, _ := callRandomProbe(t, ctx, mod, 16)
	if errno == 0 {
		t.Error("random_probe() errno = 0, want a nonzero errno (-x random must block random_get)")
	}
}

// -x未指定（既定）では実時刻(clock_time_get realtime)がtime.Now()と近い
// 値になること（wazeroの既定は偽の単調時計であり、何もしなければ仕様
// (§8.2 既定許可)と逆転する）。
func TestPolicy_clock_defaultIsRealWalltime(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	p := Policy{}
	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, p.ModuleConfig().WithStartFunctions())
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	errno, ns := callClockProbe(t, ctx, mod, 0)
	if errno != 0 {
		t.Fatalf("clock_probe(realtime) errno = %d, want 0", errno)
	}

	got := time.Unix(0, int64(ns))
	if diff := time.Since(got); diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("clock_probe(realtime) = %v, want within 5s of now (%v)", got, time.Now())
	}
}

// -x timeはWASIのclock_time_getにエラー経路がないため、「取得を遮断」を
// 表現できない。代わりにwazeroの既定（偽の単調時計、2022-01-01T00:00:00Z付近から始まり
// 読むたびに1msずつ進むだけ）のままにすることで、少なくとも実時刻を
// 見せないという実効的な効果を確認する。
func TestPolicy_clock_denyKeepsTheFakeClock(t *testing.T) {
	ctx, rt, wasmBytes := newWASIRuntime(t)

	p := Policy{Deny: Deny{Time: true}}
	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, p.ModuleConfig().WithStartFunctions())
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	errno, ns := callClockProbe(t, ctx, mod, 0)
	if errno != 0 {
		t.Fatalf("clock_probe(realtime) errno = %d, want 0 (WASI has no error path for a denied clock)", errno)
	}

	got := time.Unix(0, int64(ns))
	if diff := time.Since(got); diff < time.Hour {
		t.Errorf("clock_probe(realtime) = %v, want far in the past (wazero's fake walltime, not the real time %v)", got, time.Now())
	}
}

// フェーズ②Step7: Policy{Timeout: ...}.RuntimeConfig()が実際に
// WithCloseOnContextDoneを有効化し、recvでブロックしているゲストを
// deadline経過で強制終了できることを確認する。Step1のスパイク検証
// （timeout_spike_test.go）は生のwazero APIを直接使ったが、こちらは
// main.goが実際に呼ぶPolicy経由の配線を検証する。
func TestPolicy_runtimeConfig_timeoutInterruptsBlockingGuest(t *testing.T) {
	ctx := context.Background()

	const deadline = 200 * time.Millisecond
	const safetyLimit = 5 * time.Second

	p := Policy{Timeout: deadline, MemoryLimitBytes: 16 * wasmPageSize}
	rtConfig, err := p.RuntimeConfig()
	if err != nil {
		t.Fatalf("RuntimeConfig: %v", err)
	}

	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	mailbox := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))
	if _, err := RegisterHostModule(ctx, rt, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      NewLogger(&bytes.Buffer{}, false),
	}); err != nil {
		t.Fatalf("RegisterHostModule: %v", err)
	}

	wasmBytes, err := os.ReadFile("../testdata/modules/blocker.wasm")
	if err != nil {
		t.Fatalf("read blocker.wasm: %v", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	result := make(chan error, 1)
	go func() {
		_, err := rt.InstantiateWithConfig(runCtx, wasmBytes, wazero.NewModuleConfig())
		result <- err
	}()

	select {
	case err := <-result:
		var exitErr *sys.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("err = %v (%T), want *sys.ExitError", err, err)
		}
		if exitErr.ExitCode() != sys.ExitCodeDeadlineExceeded {
			t.Errorf("ExitCode() = %#x, want ExitCodeDeadlineExceeded (%#x)", exitErr.ExitCode(), sys.ExitCodeDeadlineExceeded)
		}
	case <-time.After(safetyLimit):
		t.Fatalf("InstantiateWithConfig did not return within the safety limit (%v)", safetyLimit)
	}
}
