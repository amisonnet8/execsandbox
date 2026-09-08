package sandbox

// 仕様書§5のWASM ABI（send/recv/max_frame）をwazeroのホストモジュール
// "execsandbox" として登録する。conn_writeはフェーズ③で追加する。

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// frameLogInterval はフレーム長超過ログの出力間隔（tail-dropと同じレート制限
// 方針。.claude/rules/cli-output.md参照）。
const frameLogInterval = 5 * time.Second

// HostConfig は execsandbox ホストモジュールの動作を決めるパラメータ。
//
// フェーズ①Step3時点ではCLIオプション（-b/--mailbox-limit、-f/--max-frame）を
// 実装していないため、呼び出し側（cmd/execsandbox）が仕様書§7.1の既定値を
// 決め打ちで渡す。フェーズ②でオプション解析の結果をここに渡す形に置き換える。
type HostConfig struct {
	// Mailbox はこのサンドボックスの受信箱。recvはここから取り出す。
	Mailbox *Mailbox
	// MaxFrame は1通あたりの最大バイト数（仕様書§5.5 max_frame、§3.4）。
	MaxFrame int
	// Log はホスト側ログ（tail-drop、フレーム長超過）の出力先。
	Log *Logger
	// Dest は起動時オプション(-d)による宛先番号の割り当てと、宛先ごとの
	// 遅延接続を保持する。nilの場合はすべての宛先が未割り当てとして扱われる
	// （-nも-dも指定しない、送信専用でも受信専用でもない構成に相当）。
	Dest *DestTable
}

// RegisterHostModule はcfgに基づき"execsandbox"モジュールをwazeroへ登録する。
func RegisterHostModule(ctx context.Context, rt wazero.Runtime, cfg HostConfig) (api.Module, error) {
	frameLog := &rateLimitedCounter{interval: frameLogInterval}

	return rt.NewHostModuleBuilder("execsandbox").
		NewFunctionBuilder().
		WithFunc(cfg.sendFunc(frameLog)).
		Export("send").
		NewFunctionBuilder().
		WithFunc(cfg.recvFunc()).
		Export("recv").
		NewFunctionBuilder().
		WithFunc(cfg.maxFrameFunc()).
		Export("max_frame").
		Instantiate(ctx)
}

// send(dest, ptr, len)。仕様書§5.2・§3.4。
//
// 戻り値を持たず、以下の場合いずれも黙ってメッセージを破棄する。
//   - 送信データ長がMaxFrameを超える（ただしこちらはログに残す）
//   - 宛先番号が未割り当て、または宛先が未起動・接続不可（DestTable.Sendの責務）
func (cfg HostConfig) sendFunc(frameLog *rateLimitedCounter) func(ctx context.Context, mod api.Module, dest, ptr, length uint32) {
	return func(ctx context.Context, mod api.Module, dest, ptr, length uint32) {
		if int(length) > cfg.MaxFrame {
			maxFrame := cfg.MaxFrame
			frameLog.Hit(time.Now(), func(n uint64) {
				cfg.Log.Printf("dropped %d oversized frame(s) (max %d bytes)", n, maxFrame)
			})
			return
		}

		data, ok := mod.Memory().Read(ptr, length)
		if !ok {
			// ゲストの誤用（範囲外ポインタ）。sendは戻り値を持たないため、
			// 安全側に倒して黙って無視する。
			return
		}

		if cfg.Dest != nil {
			// dataはゲストの線形メモリを直接指すビューであり、DestTable.Sendは
			// 呼び出しの間に同期的にWriteFrameし終えるため、コピーせず渡してよい。
			cfg.Dest.Send(dest, data)
		}
	}
}

// recv(meta_ptr, buf_ptr, buf_cap, timeout_ms) -> i32。仕様書§5.3。
//
// フェーズ①Step3時点ではtimeout_msの意味論（負値=無限待ち、0=即時、
// 正値=期限付き）はすべて実装済みだが、外部からの強制キャンセル
// （-t等によるctx経由の中断）との連携はフェーズ②で扱う
// （PLAN.md「保留事項」参照）。
func (cfg HostConfig) recvFunc() func(ctx context.Context, mod api.Module, metaPtr, bufPtr, bufCap uint32, timeoutMs int32) int32 {
	return func(ctx context.Context, mod api.Module, metaPtr, bufPtr, bufCap uint32, timeoutMs int32) int32 {
		memSize := uint64(mod.Memory().Size())
		if uint64(metaPtr)+8 > memSize || uint64(bufPtr)+uint64(bufCap) > memSize {
			// ゲストの誤用（範囲外ポインタ）。メールボックスには一切触れず諦める。
			return -1
		}

		waitCtx := ctx
		if timeoutMs >= 0 {
			var cancel context.CancelFunc
			waitCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
			defer cancel()
		}

		data, requiredLen, timedOut := cfg.Mailbox.Recv(waitCtx, int(bufCap))
		if timedOut {
			return -1
		}
		if data == nil {
			return -(int32(requiredLen) + 1)
		}

		var meta [8]byte
		binary.LittleEndian.PutUint32(meta[0:4], 0) // kind=0: サンドボックス間メッセージ
		binary.LittleEndian.PutUint32(meta[4:8], 0) // conn_id: 未使用
		mod.Memory().Write(metaPtr, meta[:])
		mod.Memory().Write(bufPtr, data)
		return int32(len(data))
	}
}

// max_frame() -> i32。仕様書§5.5。
func (cfg HostConfig) maxFrameFunc() func() int32 {
	return func() int32 {
		return int32(cfg.MaxFrame)
	}
}
