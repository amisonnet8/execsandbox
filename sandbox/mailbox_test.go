package sandbox

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestMailbox_fifoOrder(t *testing.T) {
	mb := NewMailbox(10, &bytes.Buffer{})
	mb.Push([]byte("first"))
	mb.Push([]byte("second"))

	ctx := context.Background()
	got1, _, timedOut := mb.Recv(ctx, 64)
	if timedOut || string(got1) != "first" {
		t.Fatalf("Recv #1 = %q, timedOut=%v, want %q", got1, timedOut, "first")
	}
	got2, _, timedOut := mb.Recv(ctx, 64)
	if timedOut || string(got2) != "second" {
		t.Fatalf("Recv #2 = %q, timedOut=%v, want %q", got2, timedOut, "second")
	}
}

func TestMailbox_tailDrop(t *testing.T) {
	var logBuf bytes.Buffer
	mb := NewMailbox(2, &logBuf)

	mb.Push([]byte("a"))
	mb.Push([]byte("b"))
	mb.Push([]byte("c")) // 上限(2)を超えるためtail-dropされる

	ctx := context.Background()
	got1, _, _ := mb.Recv(ctx, 64)
	got2, _, _ := mb.Recv(ctx, 64)
	if string(got1) != "a" || string(got2) != "b" {
		t.Fatalf("existing messages must survive tail-drop, got %q, %q", got1, got2)
	}

	// 3件目はキューに入らないため、4件目を送ると即座にrecvできるはず
	// （"c"がキューに残っていればこちらが先に出てしまう）。
	mb.Push([]byte("d"))
	got3, _, _ := mb.Recv(ctx, 64)
	if string(got3) != "d" {
		t.Fatalf("Recv after drop = %q, want %q (dropped message must not reappear)", got3, "d")
	}

	if !strings.Contains(logBuf.String(), "execsandbox:") {
		t.Errorf("drop log = %q, want it to contain the execsandbox: prefix", logBuf.String())
	}
}

func TestMailbox_bufferTooSmall_messageStays(t *testing.T) {
	mb := NewMailbox(4, &bytes.Buffer{})
	mb.Push([]byte("hello world"))

	ctx := context.Background()
	data, requiredLen, timedOut := mb.Recv(ctx, 4)
	if data != nil || timedOut {
		t.Fatalf("Recv with small buffer: data=%q timedOut=%v, want data=nil timedOut=false", data, timedOut)
	}
	if requiredLen != len("hello world") {
		t.Errorf("requiredLen = %d, want %d", requiredLen, len("hello world"))
	}

	// メッセージはメールボックスに残っているはずなので、十分なバッファで取り出せる。
	data, requiredLen, timedOut = mb.Recv(ctx, 64)
	if timedOut || requiredLen != 0 || string(data) != "hello world" {
		t.Fatalf("Recv with large buffer: data=%q requiredLen=%d timedOut=%v", data, requiredLen, timedOut)
	}
}

func TestMailbox_recvTimeout(t *testing.T) {
	mb := NewMailbox(4, &bytes.Buffer{})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	data, _, timedOut := mb.Recv(ctx, 64)
	if data != nil || !timedOut {
		t.Fatalf("Recv on empty mailbox = data=%q timedOut=%v, want timedOut=true", data, timedOut)
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Errorf("Recv returned too early (elapsed=%v), expected to wait for the context deadline", elapsed)
	}
}

func TestMailbox_recvUnblocksOnPush(t *testing.T) {
	mb := NewMailbox(4, &bytes.Buffer{})

	go func() {
		time.Sleep(10 * time.Millisecond)
		mb.Push([]byte("late"))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	data, _, timedOut := mb.Recv(ctx, 64)
	if timedOut || string(data) != "late" {
		t.Fatalf("Recv = data=%q timedOut=%v, want data=%q timedOut=false", data, timedOut, "late")
	}
}
