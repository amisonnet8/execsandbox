package sandbox

// フェーズ①Step1のスパイク検証。
//
// 仕様書§5のABI（send/recvのシグネチャ、meta_ptrの8バイトレイアウト）が
// wazero上で成立するかを確認する。検証対象は testdata/modules/spike.wat
// （生のABIを直接importする、SDKを介さない手書きモジュール）。
//
// ここで確認できたパターンは、フェーズ①Step3で sandbox パッケージ本体の
// ホスト関数実装として作り込む。このテストはあくまで技術検証であり、
// 本実装（メールボックス連携等）はまだ持たない。

import (
	"context"
	"encoding/binary"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func TestSpikeABI(t *testing.T) {
	ctx := context.Background()

	wasmBytes, err := os.ReadFile("../testdata/modules/spike.wasm")
	if err != nil {
		t.Fatalf("read spike.wasm: %v", err)
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	var gotSendDest uint32
	var gotSendPayload []byte

	const wantConnID = uint32(42)
	wantRecvPayload := []byte("world")

	_, err = runtime.NewHostModuleBuilder("execsandbox").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, dest, ptr, length uint32) {
			// 仕様書§5.2 send(dest, ptr, len)。
			// ゲストの線形メモリをホストが正しく読めるかを検証する。
			buf, ok := mod.Memory().Read(ptr, length)
			if !ok {
				t.Fatalf("send: failed to read guest memory at ptr=%d len=%d", ptr, length)
			}
			gotSendDest = dest
			gotSendPayload = append([]byte(nil), buf...)
		}).
		Export("send").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, metaPtr, bufPtr, bufCap uint32, timeoutMs int32) int32 {
			// 仕様書§5.3 recv(meta_ptr, buf_ptr, buf_cap, timeout_ms) -> i32。
			// meta_ptr へ [kind: u32 LE][conn_id: u32 LE] を書き込み、
			// buf_ptr へペイロードを書き込めるかを検証する。
			if timeoutMs >= 0 {
				t.Fatalf("recv: expected timeout_ms=-1 (infinite wait), got %d", timeoutMs)
			}
			if uint32(len(wantRecvPayload)) > bufCap {
				t.Fatalf("recv: payload larger than guest buffer capacity")
			}

			var meta [8]byte
			// kind=0（サンドボックス間メッセージ）、conn_id=wantConnID。
			binary.LittleEndian.PutUint32(meta[0:4], 0)
			binary.LittleEndian.PutUint32(meta[4:8], wantConnID)
			if !mod.Memory().Write(metaPtr, meta[:]) {
				t.Fatalf("recv: failed to write meta at metaPtr=%d", metaPtr)
			}
			if !mod.Memory().Write(bufPtr, wantRecvPayload) {
				t.Fatalf("recv: failed to write payload at bufPtr=%d", bufPtr)
			}
			return int32(len(wantRecvPayload))
		}).
		Export("recv").
		Instantiate(ctx)
	if err != nil {
		t.Fatalf("instantiate host module: %v", err)
	}

	guest, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("instantiate guest module: %v", err)
	}

	run := guest.ExportedFunction("run")
	if run == nil {
		t.Fatalf("guest module does not export \"run\"")
	}

	results, err := run.Call(ctx)
	if err != nil {
		t.Fatalf("call run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("run: expected 1 result, got %d", len(results))
	}

	// --- send経路の検証 ---
	if gotSendDest != 1 {
		t.Errorf("send: dest = %d, want 1", gotSendDest)
	}
	if string(gotSendPayload) != "hello" {
		t.Errorf("send: payload = %q, want %q", gotSendPayload, "hello")
	}

	// --- recv経路の検証（戻り値） ---
	gotLen := int32(results[0])
	if gotLen != int32(len(wantRecvPayload)) {
		t.Errorf("run() = %d, want %d", gotLen, len(wantRecvPayload))
	}

	// --- recv経路の検証（ゲスト線形メモリへの書き込み結果） ---
	mem := guest.Memory()

	metaBytes, ok := mem.Read(64, 8)
	if !ok {
		t.Fatalf("read meta region: out of range")
	}
	gotKind := binary.LittleEndian.Uint32(metaBytes[0:4])
	gotConnID := binary.LittleEndian.Uint32(metaBytes[4:8])
	if gotKind != 0 {
		t.Errorf("meta.kind = %d, want 0", gotKind)
	}
	if gotConnID != wantConnID {
		t.Errorf("meta.conn_id = %d, want %d", gotConnID, wantConnID)
	}

	bufBytes, ok := mem.Read(128, uint32(len(wantRecvPayload)))
	if !ok {
		t.Fatalf("read buf region: out of range")
	}
	if string(bufBytes) != string(wantRecvPayload) {
		t.Errorf("buf = %q, want %q", bufBytes, wantRecvPayload)
	}
}
