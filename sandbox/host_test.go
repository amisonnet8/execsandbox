package sandbox

// RegisterHostModuleが登録する本実装のsend/recv/max_frameを、
// testdata/modules/host_probe.wat 経由で検証する。
// Step1のスパイク（wasm_spike_test.go）はホスト関数を仮実装した技術検証で
// あったのに対し、こちらは本実装（Mailbox・レート制限ログ含む）を対象とする。

import (
	"bytes"
	"context"
	"io"
	"net"
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
		Mailbox:  NewMailbox(4, NewLogger(&bytes.Buffer{}, false)),
		MaxFrame: 12345,
		Log:      NewLogger(&bytes.Buffer{}, false),
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
		Mailbox:  NewMailbox(4, NewLogger(&logBuf, false)),
		MaxFrame: 1024,
		Log:      NewLogger(&logBuf, false),
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
		Mailbox:  NewMailbox(4, NewLogger(&logBuf, false)),
		MaxFrame: 100,
		Log:      NewLogger(&logBuf, false),
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
		Mailbox:  NewMailbox(4, NewLogger(&bytes.Buffer{}, false)),
		MaxFrame: 1024,
		Log:      NewLogger(&bytes.Buffer{}, false),
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
	mailbox := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))
	mailbox.Push(Message{Payload: []byte("a message longer than four bytes")})

	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      NewLogger(&bytes.Buffer{}, false),
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
	mailbox := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))
	ctx, _, guest := newProbeGuest(t, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      NewLogger(&bytes.Buffer{}, false),
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		mailbox.Push(Message{Payload: []byte("ping")})
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
	mailbox := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	if _, err := RegisterHostModule(ctx, rt, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      NewLogger(&bytes.Buffer{}, false),
	}); err != nil {
		t.Fatalf("RegisterHostModule: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		mailbox.Push(Message{Payload: []byte("pong")})
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

// TestHostModule_connWrite_echoesExternalData は、conn_write（仕様書§5.4）を
// testdata/modules/conn_echo.wat経由でエンドツーエンドに検証する。実際の
// TCPリスナー・クライアントを使い、-l/--listen経由で受け付けた接続の
// データ(kind=2)が、ゲストのconn_writeでそのまま書き戻されることを確認する。
//
// conn_echo.wasmの"_start"はrecvの無限ループで、外部から取り消されない限り
// 自発的には終了しない（blocker.wasmと同じ構造）。そのためテスト用contextに
// deadlineを持たせ、wazero.RuntimeConfig.WithCloseOnContextDone(true)で
// 強制終了させることでテストを終わらせる(sandbox/timeout_spike_test.goと
// 同じ手口)。
func TestHostModule_connWrite_echoesExternalData(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer l.Close()

	mailbox := NewMailbox(16, NewLogger(&bytes.Buffer{}, false))
	connTable := NewConnTable(mailbox)
	defer connTable.Close()
	go connTable.Serve(l, 1024)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rtConfig := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(context.Background())

	if _, err := RegisterHostModule(ctx, rt, HostConfig{
		Mailbox:  mailbox,
		MaxFrame: 1024,
		Log:      NewLogger(&bytes.Buffer{}, false),
		Conns:    connTable,
	}); err != nil {
		t.Fatalf("RegisterHostModule: %v", err)
	}

	wasmBytes, err := os.ReadFile("../testdata/modules/conn_echo.wasm")
	if err != nil {
		t.Fatalf("read conn_echo.wasm: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := rt.Instantiate(ctx, wasmBytes)
		done <- err
	}()

	client, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer client.Close()

	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("client.Write: %v", err)
	}

	client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatalf("client.Read (echo): %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("echoed data = %q, want %q", buf, "ping")
	}

	// ゲストは無限ループなので、テストを終えるために明示的にcontextを
	// 取り消し、WithCloseOnContextDoneによる強制終了を待つ。
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("guest did not stop after context cancellation")
	}
}
