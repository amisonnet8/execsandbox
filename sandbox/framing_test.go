package sandbox

import (
	"bytes"
	"io"
	"testing"
)

func TestWriteReadFrame_roundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, []byte("hello")); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if err := WriteFrame(&buf, []byte("world!")); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	data, oversized, err := ReadFrame(&buf, 1024)
	if err != nil || oversized || string(data) != "hello" {
		t.Fatalf("ReadFrame #1 = %q, oversized=%v, err=%v", data, oversized, err)
	}
	data, oversized, err = ReadFrame(&buf, 1024)
	if err != nil || oversized || string(data) != "world!" {
		t.Fatalf("ReadFrame #2 = %q, oversized=%v, err=%v", data, oversized, err)
	}
}

func TestReadFrame_oversizedIsDiscardedAndStreamStaysInSync(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, bytes.Repeat([]byte{0xAA}, 100)); err != nil {
		t.Fatalf("WriteFrame (oversized): %v", err)
	}
	if err := WriteFrame(&buf, []byte("next")); err != nil {
		t.Fatalf("WriteFrame (next): %v", err)
	}

	data, oversized, err := ReadFrame(&buf, 10)
	if err != nil || !oversized || data != nil {
		t.Fatalf("ReadFrame (oversized) = %q, oversized=%v, err=%v, want oversized=true", data, oversized, err)
	}

	data, oversized, err = ReadFrame(&buf, 10)
	if err != nil || oversized || string(data) != "next" {
		t.Fatalf("ReadFrame (after oversized) = %q, oversized=%v, err=%v, want %q", data, oversized, err, "next")
	}
}

func TestReadFrame_eof(t *testing.T) {
	_, _, err := ReadFrame(bytes.NewReader(nil), 1024)
	if err != io.EOF {
		t.Fatalf("ReadFrame on empty reader: err = %v, want io.EOF", err)
	}
}
