package main

import (
	"bytes"
	"testing"

	"github.com/amisonnet8/execsandbox/sandbox"
)

// TestStamp_roundTrip は、stamp()が書いたバイト列を、読み出し側の実装
// (sandbox.ExtractWASM)で正しく読み戻せることを確認する。書き込み側・
// 読み出し側が独立実装であるがゆえの整合性チェック
// (.claude/rules/directory-structure.md)。
func TestStamp_roundTrip(t *testing.T) {
	base := []byte("dummy base binary bytes")
	wasm := []byte("dummy wasm module bytes")

	out := stamp(base, wasm)

	if !bytes.HasPrefix(out, base) {
		t.Fatalf("stamped output does not start with the base binary bytes")
	}

	got, err := sandbox.ExtractWASM(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("sandbox.ExtractWASM: %v", err)
	}
	if !bytes.Equal(got, wasm) {
		t.Errorf("ExtractWASM() = %q, want %q", got, wasm)
	}
}

func TestStamp_doesNotMutateInputs(t *testing.T) {
	base := []byte("base")
	wasm := []byte("wasm")
	baseCopy := append([]byte(nil), base...)
	wasmCopy := append([]byte(nil), wasm...)

	_ = stamp(base, wasm)

	if !bytes.Equal(base, baseCopy) {
		t.Error("stamp() mutated base")
	}
	if !bytes.Equal(wasm, wasmCopy) {
		t.Error("stamp() mutated wasm")
	}
}

func TestFinalOutputPath(t *testing.T) {
	tests := []struct {
		name   string
		output string
		goos   string
		want   string
	}{
		{name: "linux unchanged", output: "mydb", goos: "linux", want: "mydb"},
		{name: "darwin unchanged", output: "mydb", goos: "darwin", want: "mydb"},
		{name: "windows appends .exe", output: "mydb", goos: "windows", want: "mydb.exe"},
		{name: "windows already has .exe", output: "mydb.exe", goos: "windows", want: "mydb.exe"},
		{name: "windows already has .EXE (case-insensitive)", output: "mydb.EXE", goos: "windows", want: "mydb.EXE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &options{output: tt.output, target: target{GOOS: tt.goos, GOARCH: "amd64"}}
			if got := finalOutputPath(opts); got != tt.want {
				t.Errorf("finalOutputPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
