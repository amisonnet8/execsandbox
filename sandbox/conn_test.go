package sandbox

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

// recvExpect はmb.Recvで1件取り出し、outcomeがRecvDeliveredでなければ
// テストを失敗させるヘルパー。
func recvExpect(t *testing.T, mb *Mailbox, timeout time.Duration) Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	msg, _, outcome := mb.Recv(ctx, 65536)
	if outcome != RecvDelivered {
		t.Fatalf("Recv: outcome = %v, want RecvDelivered", outcome)
	}
	return msg
}

func TestConnTable_establishDataDisconnect(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer l.Close()

	mb := NewMailbox(16, NewLogger(&bytes.Buffer{}, false))
	ct := NewConnTable()
	defer ct.Close()
	go ct.Serve(l, mb, 1024)

	client, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}

	established := recvExpect(t, mb, time.Second)
	if established.Kind != 1 || len(established.Payload) != 0 {
		t.Fatalf("establish event = %+v, want kind=1 with no payload", established)
	}
	connID := established.ConnID
	if connID == 0 {
		t.Fatalf("establish event conn_id = 0, want nonzero (0 is reserved for kind=0)")
	}

	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatalf("client.Write: %v", err)
	}
	data := recvExpect(t, mb, time.Second)
	if data.Kind != 2 || data.ConnID != connID || string(data.Payload) != "hello" {
		t.Fatalf("data event = %+v, want kind=2 conn_id=%d payload=%q", data, connID, "hello")
	}

	client.Close()
	disconnect := recvExpect(t, mb, time.Second)
	if disconnect.Kind != 3 || disconnect.ConnID != connID || len(disconnect.Payload) != 0 {
		t.Fatalf("disconnect event = %+v, want kind=3 conn_id=%d with no payload", disconnect, connID)
	}
}

func TestConnTable_connIDsAreNotReused(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer l.Close()

	mb := NewMailbox(16, NewLogger(&bytes.Buffer{}, false))
	ct := NewConnTable()
	defer ct.Close()
	go ct.Serve(l, mb, 1024)

	// 1本目の接続を確立してすぐ閉じる。
	c1, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial #1: %v", err)
	}
	established1 := recvExpect(t, mb, time.Second)
	c1.Close()
	disconnect1 := recvExpect(t, mb, time.Second)
	if disconnect1.ConnID != established1.ConnID {
		t.Fatalf("disconnect1.ConnID = %d, want %d", disconnect1.ConnID, established1.ConnID)
	}

	// 2本目を確立し、connIDが1本目と異なる(再利用されない)ことを確認する。
	c2, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial #2: %v", err)
	}
	defer c2.Close()
	established2 := recvExpect(t, mb, time.Second)
	if established2.ConnID == established1.ConnID {
		t.Errorf("established2.ConnID = %d, want different from established1.ConnID = %d (must not reuse)", established2.ConnID, established1.ConnID)
	}
	if established2.ConnID <= established1.ConnID {
		t.Errorf("established2.ConnID = %d, want greater than established1.ConnID = %d (monotonically increasing)", established2.ConnID, established1.ConnID)
	}
}

func TestConnTable_disconnectBypassesMailboxLimit(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer l.Close()

	// 上限0のメールボックスを使う。通常のPush(確立イベント等)はキューの
	// 状態に関わらず常にtail-dropされるため、「切断イベントだけが上限を
	// 無視して必ず届く」ことを、事前にメッセージを積んでおく手法より
	// 競合なく決定的に検証できる(事前投入方式は、recvでの取り出しと
	// 確立イベントのpushが競合し、タイミング次第でどちらが先にキューへ
	// 入るか不定になってしまうため採らない)。
	var logBuf bytes.Buffer
	mb := NewMailbox(0, NewLogger(&logBuf, false))

	ct := NewConnTable()
	defer ct.Close()
	go ct.Serve(l, mb, 1024)

	// このConnTableで最初に受け付ける接続なのでconnIDは1になる。
	client, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	const wantConnID = 1
	client.Close()

	// 確立イベント(kind=1)は常にtail-dropされ、切断イベント(kind=3)だけが
	// 上限を無視して必ず積まれているはず。
	disconnect := recvExpect(t, mb, time.Second)
	if disconnect.Kind != 3 || disconnect.ConnID != wantConnID {
		t.Fatalf("disconnect event = %+v, want kind=3 conn_id=%d despite a zero-capacity mailbox", disconnect, wantConnID)
	}
}
