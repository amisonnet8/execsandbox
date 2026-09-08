package sandbox

// フッターの読み出し(ExtractWASM)を検証する。
//
// 書き込み側（ビルダー相当のフッター組み立て）はcmd/execsandbox-buildの責務
// であり本パッケージには置かない（.claude/rules/directory-structure.md）。
// ここではテスト用に、フッター付きバイト列を組み立てる非公開ヘルパーのみを
// 用意する。

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// buildStamped は「ベースバイナリ + WASMモジュール + フッター」の完成形バイト列を
// 組み立てる。実際のビルダー（フェーズ④）が行う結合をテスト用に代替するもの。
func buildStamped(t *testing.T, base, wasm []byte) []byte {
	t.Helper()

	footer := make([]byte, footerSize)
	copy(footer[0:8], footerMagic)
	binary.BigEndian.PutUint32(footer[8:12], footerVersion)
	binary.BigEndian.PutUint64(footer[12:20], uint64(len(base)))
	binary.BigEndian.PutUint64(footer[20:28], uint64(len(wasm)))
	// footer[28:32] は Reserved（ゼロのまま）。

	out := make([]byte, 0, len(base)+len(wasm)+footerSize)
	out = append(out, base...)
	out = append(out, wasm...)
	out = append(out, footer...)
	return out
}

func TestExtractWASM_success(t *testing.T) {
	base := bytes.Repeat([]byte{0xAA}, 1000)
	wasm := []byte("\x00asm-fake-module-bytes")
	stamped := buildStamped(t, base, wasm)

	// stamp.md: スタンプのテストはビルド成果物ではなくコピー（一時ファイル）に対して行う。
	path := filepath.Join(t.TempDir(), "stamped-copy")
	if err := os.WriteFile(path, stamped, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}

	got, err := ExtractWASM(f, stat.Size())
	if err != nil {
		t.Fatalf("ExtractWASM: %v", err)
	}
	if !bytes.Equal(got, wasm) {
		t.Errorf("ExtractWASM = %q, want %q", got, wasm)
	}
}

func TestExtractWASM_notStamped(t *testing.T) {
	t.Run("too small for a footer", func(t *testing.T) {
		raw := []byte("tiny")
		_, err := ExtractWASM(bytes.NewReader(raw), int64(len(raw)))
		if !errors.Is(err, ErrNotStamped) {
			t.Fatalf("ExtractWASM error = %v, want ErrNotStamped", err)
		}
	})

	t.Run("magic mismatch", func(t *testing.T) {
		raw := bytes.Repeat([]byte{0x00}, 500) // フッターサイズはあるがマジックなし
		_, err := ExtractWASM(bytes.NewReader(raw), int64(len(raw)))
		if !errors.Is(err, ErrNotStamped) {
			t.Fatalf("ExtractWASM error = %v, want ErrNotStamped", err)
		}
	})
}

func TestExtractWASM_corruptFooter(t *testing.T) {
	base := bytes.Repeat([]byte{0xAA}, 100)
	wasm := []byte("real-wasm-bytes")
	stamped := buildStamped(t, base, wasm)

	// Lengthフィールドを改ざんし、offset+lengthがファイルサイズと矛盾する状態を作る。
	binary.BigEndian.PutUint64(stamped[len(stamped)-footerSize+20:], uint64(len(wasm)+9999))

	_, err := ExtractWASM(bytes.NewReader(stamped), int64(len(stamped)))
	if err == nil {
		t.Fatal("ExtractWASM: expected error for corrupt footer, got nil")
	}
	if errors.Is(err, ErrNotStamped) {
		t.Fatalf("ExtractWASM error = %v, want a corruption error (not ErrNotStamped)", err)
	}
}
