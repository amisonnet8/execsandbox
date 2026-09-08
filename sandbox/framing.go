package sandbox

// サンドボックス間メッセージのフレーミング（仕様書§3.5）。
//
//	[長さ: u32 リトルエンディアン][データ本体]
//
// リトルエンディアンとするのはWASMの線形メモリ表現に合わせるため
// （スタンプのフッターがビッグエンディアンなのとは別の理由に基づく。
// .claude/rules/stamp.md参照）。

import (
	"encoding/binary"
	"io"
)

// WriteFrame は長さプレフィックス付きで1通を書き込む。
// maxFrameの検証は呼び出し側（ホスト関数のsend実装）の責務とする。
func WriteFrame(w io.Writer, data []byte) error {
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// ReadFrame は1通を読み取る。
//
//   - 通常時: data にペイロードを、oversized=false, err=nil で返す。
//   - 宣言された長さがmaxFrameを超える場合: 仕様書§3.5により受理せず破棄する。
//     ペイロード分をストリームから読み捨てて同期を保ち、data=nil,
//     oversized=true, err=nil を返す（呼び出し側はログを出して次のフレームを
//     読み続けてよい）。
//   - 接続断・読み取りエラーの場合: errを返す。
func ReadFrame(r io.Reader, maxFrame int) (data []byte, oversized bool, err error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, false, err
	}
	n := binary.LittleEndian.Uint32(lenBuf[:])

	if int64(n) > int64(maxFrame) {
		if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, false, err
	}
	return buf, false, nil
}
