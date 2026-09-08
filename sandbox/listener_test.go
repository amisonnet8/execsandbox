package sandbox

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListenAndDestTable_roundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	mailbox := NewMailbox(4, &bytes.Buffer{})
	l, err := Listen("nodeB")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	go Serve(l, mailbox, 1024, &bytes.Buffer{})

	dest := NewDestTable(map[uint32]string{1: "nodeB"})
	defer dest.Close()

	dest.Send(1, []byte("hello from nodeA"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, _, timedOut := mailbox.Recv(ctx, 1024)
	if timedOut || string(got) != "hello from nodeA" {
		t.Fatalf("mailbox.Recv() = %q, timedOut=%v", got, timedOut)
	}
}

func TestListen_staleSocketIsCleanedUp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	// 前回のプロセスが残したstaleなソケットファイルを模倣する
	// （誰も待ち受けていない、ただのファイル）。
	socketDir := filepath.Join(dir, "execsandbox")
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	stalePath := filepath.Join(socketDir, "nodeC.sock")
	// 実際にstaleなAF_UNIXファイルを作るため、一度Listenしてから
	// Close()（accept前提のファイルだけ残る状況を再現）する。
	tmp, err := net.Listen("unix", stalePath)
	if err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	tmp.Close() // ファイルは残るが、誰も待ち受けていない状態になる

	l, err := Listen("nodeC")
	if err != nil {
		t.Fatalf("Listen should clean up the stale socket and succeed: %v", err)
	}
	l.Close()
}

func TestListen_rejectsWhenAlreadyListening(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	l1, err := Listen("nodeD")
	if err != nil {
		t.Fatalf("Listen (first): %v", err)
	}
	defer l1.Close()

	_, err = Listen("nodeD")
	if err == nil {
		t.Fatal("Listen (second, same ID) should fail while the first is still listening")
	}
}

func TestDestTable_unassignedDestinationIsSilentlyDropped(t *testing.T) {
	dest := NewDestTable(nil)
	defer dest.Close()
	dest.Send(99, []byte("nobody is listening for this")) // パニックしないことだけを確認する
}
