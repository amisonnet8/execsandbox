package sandbox

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestMailbox_fifoOrder(t *testing.T) {
	mb := NewMailbox(10, NewLogger(&bytes.Buffer{}, false))
	mb.Push(Message{Payload: []byte("first")})
	mb.Push(Message{Payload: []byte("second")})

	ctx := context.Background()
	got1, _, outcome := mb.Recv(ctx, 64)
	if outcome != RecvDelivered || string(got1.Payload) != "first" {
		t.Fatalf("Recv #1 = %q, outcome=%v, want %q", got1.Payload, outcome, "first")
	}
	got2, _, outcome := mb.Recv(ctx, 64)
	if outcome != RecvDelivered || string(got2.Payload) != "second" {
		t.Fatalf("Recv #2 = %q, outcome=%v, want %q", got2.Payload, outcome, "second")
	}
}

func TestMailbox_tailDrop(t *testing.T) {
	var logBuf bytes.Buffer
	mb := NewMailbox(2, NewLogger(&logBuf, false))

	mb.Push(Message{Payload: []byte("a")})
	mb.Push(Message{Payload: []byte("b")})
	mb.Push(Message{Payload: []byte("c")}) // 上限(2)を超えるためtail-dropされる

	ctx := context.Background()
	got1, _, _ := mb.Recv(ctx, 64)
	got2, _, _ := mb.Recv(ctx, 64)
	if string(got1.Payload) != "a" || string(got2.Payload) != "b" {
		t.Fatalf("existing messages must survive tail-drop, got %q, %q", got1.Payload, got2.Payload)
	}

	// 3件目はキューに入らないため、4件目を送ると即座にrecvできるはず
	// （"c"がキューに残っていればこちらが先に出てしまう）。
	mb.Push(Message{Payload: []byte("d")})
	got3, _, _ := mb.Recv(ctx, 64)
	if string(got3.Payload) != "d" {
		t.Fatalf("Recv after drop = %q, want %q (dropped message must not reappear)", got3.Payload, "d")
	}

	if !strings.Contains(logBuf.String(), "execsandbox:") {
		t.Errorf("drop log = %q, want it to contain the execsandbox: prefix", logBuf.String())
	}
}

func TestMailbox_bufferTooSmall_messageStays(t *testing.T) {
	mb := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))
	mb.Push(Message{Payload: []byte("hello world")})

	ctx := context.Background()
	msg, requiredLen, outcome := mb.Recv(ctx, 4)
	if outcome != RecvBufferTooSmall {
		t.Fatalf("Recv with small buffer: msg=%+v outcome=%v, want RecvBufferTooSmall", msg, outcome)
	}
	if requiredLen != len("hello world") {
		t.Errorf("requiredLen = %d, want %d", requiredLen, len("hello world"))
	}

	// メッセージはメールボックスに残っているはずなので、十分なバッファで取り出せる。
	msg, requiredLen, outcome = mb.Recv(ctx, 64)
	if outcome != RecvDelivered || requiredLen != 0 || string(msg.Payload) != "hello world" {
		t.Fatalf("Recv with large buffer: payload=%q requiredLen=%d outcome=%v", msg.Payload, requiredLen, outcome)
	}
}

func TestMailbox_recvTimeout(t *testing.T) {
	mb := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	msg, _, outcome := mb.Recv(ctx, 64)
	if outcome != RecvTimedOut {
		t.Fatalf("Recv on empty mailbox = msg=%+v outcome=%v, want RecvTimedOut", msg, outcome)
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Errorf("Recv returned too early (elapsed=%v), expected to wait for the context deadline", elapsed)
	}
}

func TestMailbox_recvUnblocksOnPush(t *testing.T) {
	mb := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))

	go func() {
		time.Sleep(10 * time.Millisecond)
		mb.Push(Message{Payload: []byte("late")})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg, _, outcome := mb.Recv(ctx, 64)
	if outcome != RecvDelivered || string(msg.Payload) != "late" {
		t.Fatalf("Recv = payload=%q outcome=%v, want payload=%q outcome=RecvDelivered", msg.Payload, outcome, "late")
	}
}

func TestMailbox_pushDisconnect_ignoresLimit(t *testing.T) {
	var logBuf bytes.Buffer
	mb := NewMailbox(1, NewLogger(&logBuf, false))

	// 上限(1)をすでに埋めたうえで、切断イベントを3件積む。
	// PushDisconnectは上限を無視して必ず積まれる必要がある（仕様書§4.2）。
	mb.Push(Message{Payload: []byte("a")})
	mb.PushDisconnect(1)
	mb.PushDisconnect(2)
	mb.PushDisconnect(3)

	ctx := context.Background()
	msg, _, outcome := mb.Recv(ctx, 64)
	if outcome != RecvDelivered || string(msg.Payload) != "a" {
		t.Fatalf("Recv #1 = %+v outcome=%v, want payload %q", msg, outcome, "a")
	}

	for _, wantConnID := range []uint32{1, 2, 3} {
		msg, _, outcome := mb.Recv(ctx, 64)
		if outcome != RecvDelivered || msg.Kind != 3 || msg.ConnID != wantConnID {
			t.Fatalf("Recv disconnect = %+v outcome=%v, want kind=3 conn_id=%d", msg, outcome, wantConnID)
		}
	}

	if logBuf.Len() != 0 {
		t.Errorf("log = %q, want empty (PushDisconnect must not log a drop)", logBuf.String())
	}
}
