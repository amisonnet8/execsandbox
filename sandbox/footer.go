// Package sandbox は、WASMモジュールの実行・ポリシー適用・メッセージ配送を担う。
package sandbox

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// footerMagic は末尾フッターの先頭8バイトに置く固定文字列。
	footerMagic = "EXECSB01"

	// footerSize はフッター全体のバイト数（Magic 8 + Version 4 + Offset 8 + Length 8 + Reserved 4）。
	footerSize = 32

	// footerVersion はフッター形式自体のバージョン。
	// 現時点でExtractWASMはこの値を分岐に使わないが、将来レイアウトを変更した際に
	// 古い実行ファイルを判別できるよう、書き込み側（ビルダー）が埋め込む値として残す。
	footerVersion = uint32(1)
)

// ErrNotStamped は、実行ファイルの末尾にWASMモジュールが埋め込まれていないことを示す。
// マジックが一致しない、またはフッターが収まるサイズすらない場合に返る。
var ErrNotStamped = errors.New("no WASM module embedded in this binary")

// ExtractWASM は r の末尾32バイトのフッターを読み、埋め込まれたWASMモジュールの
// バイト列を返す。size は r 全体のバイト数（呼び出し側が os.File.Stat 等で
// 取得済みの値を渡す）。
//
// ファイル全体を読み込まず ReadAt のみで完結させる。ベースバイナリは
// 10MB前後あるため。
func ExtractWASM(r io.ReaderAt, size int64) ([]byte, error) {
	if size < footerSize {
		return nil, ErrNotStamped
	}

	footer := make([]byte, footerSize)
	if _, err := r.ReadAt(footer, size-footerSize); err != nil {
		return nil, fmt.Errorf("read footer: %w", err)
	}

	if string(footer[0:8]) != footerMagic {
		return nil, ErrNotStamped
	}

	// footer[8:12] はVersion（現時点では単一版のみ存在するため未使用）。
	offset := binary.BigEndian.Uint64(footer[12:20])
	length := binary.BigEndian.Uint64(footer[20:28])

	if offset > uint64(size) || length > uint64(size)-offset || int64(offset+length) != size-footerSize {
		return nil, fmt.Errorf("corrupt footer: offset=%d length=%d does not match file size=%d", offset, length, size)
	}

	wasm := make([]byte, length)
	if _, err := r.ReadAt(wasm, int64(offset)); err != nil {
		return nil, fmt.Errorf("read embedded WASM module: %w", err)
	}
	return wasm, nil
}
