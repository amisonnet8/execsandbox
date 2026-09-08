package sandbox

// フェーズ②Step1のスパイク検証。
//
// PLAN.mdの保留事項「-tでプロセス全体を強制終了する際、recvでブロック中の
// ゲストを安全に中断する方法」を実測で確定させる。検証する経路は2つ：
//
//  1. ホスト関数(recv)の中でブロックしているゲスト（testdata/modules/blocker.wat）
//  2. ホスト関数を呼ばず純WASMのループを回しているだけのゲスト
//     （testdata/modules/spin_forever.wat）
//
// wazero.RuntimeConfig.WithCloseOnContextDone(true) を有効にした状態で、
// InstantiateWithConfigに渡すcontextがdeadlineを迎えたとき、両方の経路とも
// *sys.ExitError（ExitCode() == sys.ExitCodeDeadlineExceeded）で終了する
// ことを確認する。

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

// runWithDeadline はwasmPathのモジュールを、deadline経過で強制終了される
// 設定で実行し、そのCallの戻りエラーを返す。テスト自体がハングしないよう、
// deadlineに対して十分な安全マージンを取った上限（safetyLimit）で
// goroutine+selectを使い、それでも戻らなければテストを失敗させる。
func runWithDeadline(t *testing.T, wasmPath string, needsHostModule bool, deadline, safetyLimit time.Duration) error {
	t.Helper()

	ctx := context.Background()
	rtConfig := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	if needsHostModule {
		// メッセージは一切届かない前提のMailbox。recvが本当にブロックし
		// 続けることを保証する。
		mailbox := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))
		if _, err := RegisterHostModule(ctx, rt, HostConfig{
			Mailbox:  mailbox,
			MaxFrame: 1024,
			Log:      NewLogger(&bytes.Buffer{}, false),
		}); err != nil {
			t.Fatalf("RegisterHostModule: %v", err)
		}
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatalf("read %s: %v", wasmPath, err)
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
		return err
	case <-time.After(safetyLimit):
		t.Fatalf("InstantiateWithConfig did not return within the safety limit (%v); "+
			"WithCloseOnContextDone did not interrupt the guest as expected", safetyLimit)
		return nil // unreachable
	}
}

func TestTimeoutSpike_hostFunctionBlock(t *testing.T) {
	err := runWithDeadline(t, "../testdata/modules/blocker.wasm", true, 200*time.Millisecond, 5*time.Second)

	var exitErr *sys.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v (%T), want *sys.ExitError", err, err)
	}
	if exitErr.ExitCode() != sys.ExitCodeDeadlineExceeded {
		t.Errorf("ExitCode() = %#x, want ExitCodeDeadlineExceeded (%#x)", exitErr.ExitCode(), sys.ExitCodeDeadlineExceeded)
	}
}

func TestTimeoutSpike_pureWasmLoop(t *testing.T) {
	err := runWithDeadline(t, "../testdata/modules/spin_forever.wasm", false, 200*time.Millisecond, 5*time.Second)

	var exitErr *sys.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v (%T), want *sys.ExitError", err, err)
	}
	if exitErr.ExitCode() != sys.ExitCodeDeadlineExceeded {
		t.Errorf("ExitCode() = %#x, want ExitCodeDeadlineExceeded (%#x)", exitErr.ExitCode(), sys.ExitCodeDeadlineExceeded)
	}
}
