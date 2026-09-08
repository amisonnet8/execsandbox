package sandbox

// RegisterHostModuleが登録する本実装のsend/recv/max_frameを、
// testdata/modules/host_probe.wat 経由で検証する。
// Step1のスパイク（wasm_spike_test.go）はホスト関数を仮実装した技術検証で
// あったのに対し、こちらは本実装（Mailbox・レート制限ログ含む）を対象とする。

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func loadHostProbe(t *testing.T) []byte {
	t.Helper()
	wasmBytes, err := os.ReadFile("../testdata/modules/host_probe.wasm")
	if err != nil {
		t.Fatalf("read host_probe.wasm: %v", err)
	}
	return wasmBytes
}

// newProbeGuest はホストモジュールを登録したうえでhost_probe.wasmを
// インスタンス化する。_startは自動実行されないよう、明示的なexport呼び出しで
// 使うテスト向けに wazero.NewRuntimeConfig 既定のまま（"_start"は
// 存在するので自動実行されてしまう点に注意——このヘルパーは_startを
// 使わないテスト専用とし、_start自体のテストは別関数で行う）。
func newProbeGuest(t *testing.T, cfg HostConfig) (context.Context, wazero.Runtime, api.Module) {
	t.Helper()
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	t.Cleanup(func() { rt.Close(ctx) })

	if _, err := RegisterHostModule(ctx, rt, cfg); err != nil {
		t.Fatalf("RegisterHostModule: %v", err)
	}

	// host_probe.wasmは"_start"をexportしているため、そのままInstantiateすると
	// wazeroの既定StartFunctionsにより自動実行され、recv(timeout=-1)で
	// ブロックしてしまう。named exportのみを使うテストでは自動実行を止める。
	modCfg := wazero.NewModuleConfig().WithStartFunctions()
	guest, err := rt.InstantiateWithConfig(ctx, loadHostProbe(t), modCfg)
	if err != nil {
		t.Fatalf("instantiate host_probe.wasm: %v", err)
	}
	return ctx, rt, guest
}

func TestHostModule_maxFrame(t *testing.T) {
	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  NewMailbox(4, &bytes.Buffer{}),
		MaxFrame: 12345,
		Log:      &bytes.Buffer{},
	})

	results, err := guest.ExportedFunction("max_frame_probe").Call(ctx)
	if err != nil {
		t.Fatalf("call max_frame_probe: %v", err)
	}
	if got := int32(results[0]); got != 12345 {
		t.Errorf("max_frame_probe() = %d, want 12345", got)
	}
}

func TestHostModule_sendWithinLimit_noLog(t *testing.T) {
	var logBuf bytes.Buffer
	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  NewMailbox(4, &logBuf),
		MaxFrame: 1024,
		Log:      &logBuf,
	})

	// send_probeはオフセット0から読むので、事前に有効なペイロードが要る
	// （ゼロ埋めのメモリで十分。長さだけがsendの判定に使われる）。
	if _, err := guest.ExportedFunction("send_probe").Call(ctx, 1, 100); err != nil {
		t.Fatalf("call send_probe: %v", err)
	}

	if logBuf.Len() != 0 {
		t.Errorf("log = %q, want empty (unassigned destination must be silent per spec §3.4)", logBuf.String())
	}
}

func TestHostModule_sendOverMaxFrame_logs(t *testing.T) {
	var logBuf bytes.Buffer
	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  NewMailbox(4, &logBuf),
		MaxFrame: 100,
		Log:      &logBuf,
	})

	if _, err := guest.ExportedFunction("send_probe").Call(ctx, 1, 200); err != nil {
		t.Fatalf("call send_probe: %v", err)
	}

	got := logBuf.String()
	if !strings.Contains(got, "execsandbox:") || !strings.Contains(got, "oversized") {
		t.Errorf("log = %q, want a message about an oversized frame prefixed with execsandbox:", got)
	}
}

func TestHostModule_recvImmediateTimeout(t *testing.T) {
	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  NewMailbox(4, &bytes.Buffer{}),
		MaxFrame: 1024,
		Log:      &bytes.Buffer{},
	})

	results, err := guest.ExportedFunction("recv_probe").Call(ctx, 64, api.EncodeI32(0))
	if err != nil {
		t.Fatalf("call recv_probe: %v", err)
	}
	if got := int32(results[0]); got != -1 {
		t.Errorf("recv_probe(buf_cap=64, timeout_ms=0) on empty mailbox = %d, want -1", got)
	}
}

func TestHostModule_recvBufferTooSmall_messageStays(t *testing.T) {
	mailbox := NewMailbox(4, &bytes.Buffer{})
	mailbox.Push([]byte("a message longer than four bytes"))

	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      &bytes.Buffer{},
	})

	results, err := guest.ExportedFunction("recv_probe").Call(ctx, 4, api.EncodeI32(0))
	if err != nil {
		t.Fatalf("call recv_probe (small buffer): %v", err)
	}
	got := int32(results[0])
	wantRequired := len("a message longer than four bytes")
	if got != -(int32(wantRequired) + 1) {
		t.Fatalf("recv_probe(buf_cap=4) = %d, want %d (encoded required size)", got, -(int32(wantRequired) + 1))
	}

	// メッセージが残っているはずなので、十分なバッファなら取り出せる。
	results, err = guest.ExportedFunction("recv_probe").Call(ctx, 128, api.EncodeI32(0))
	if err != nil {
		t.Fatalf("call recv_probe (large buffer): %v", err)
	}
	if got := int32(results[0]); int(got) != wantRequired {
		t.Fatalf("recv_probe(buf_cap=128) = %d, want %d", got, wantRequired)
	}
}

func TestHostModule_recvBlocksUntilPush(t *testing.T) {
	mailbox := NewMailbox(4, &bytes.Buffer{})
	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      &bytes.Buffer{},
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		mailbox.Push([]byte("ping"))
	}()

	// timeout_ms = -1 (無限待ち)
	results, err := guest.ExportedFunction("recv_probe").Call(ctx, 64, api.EncodeI32(-1))
	if err != nil {
		t.Fatalf("call recv_probe: %v", err)
	}
	if got := int32(results[0]); int(got) != len("ping") {
		t.Fatalf("recv_probe(timeout_ms=-1) = %d, want %d", got, len("ping"))
	}

	mem := guest.Memory()
	buf, ok := mem.Read(65544, uint32(len("ping")))
	if !ok || string(buf) != "ping" {
		t.Errorf("received payload = %q, ok=%v, want %q", buf, ok, "ping")
	}
}

func TestHostModule_start_receivesAndEchoes(t *testing.T) {
	mailbox := NewMailbox(4, &bytes.Buffer{})

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	if _, err := RegisterHostModule(ctx, rt, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("RegisterHostModule: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		mailbox.Push([]byte("pong"))
	}()

	// _startが自動実行され、内部でrecv(timeout=-1)がブロックしたのち
	// mailbox.Pushで解放される。本番のcmd/execsandboxが使う経路そのもの。
	guest, err := rt.Instantiate(ctx, loadHostProbe(t))
	if err != nil {
		t.Fatalf("instantiate (auto _start): %v", err)
	}

	mem := guest.Memory()
	recvResult, ok := mem.ReadUint32Le(65524)
	if !ok {
		t.Fatalf("read recv result slot: out of range")
	}
	if int32(recvResult) != int32(len("pong")) {
		t.Errorf("_start's recv result = %d, want %d", int32(recvResult), len("pong"))
	}
}
