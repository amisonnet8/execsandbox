package sandbox

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// newTestRuntimeDir はXDG_RUNTIME_DIRベースのテストに使う短い一時ディレクトリを
// 用意し、環境変数を設定する。
//
// t.TempDir()（内部的にはos.TempDir()、macOSでは$TMPDIRの
// "/var/folders/.../T/..." という長いパス）をそのまま使うと、AF_UNIXの
// sun_path上限（Linuxは108バイト、macOSは104バイト程度と、より短い）を
// 超えて "bind: invalid argument" になることがある。実際にGitHub Actionsの
// macos-latestで踏んだため、/tmp直下に短い名前で作る。
//
// Windowsは仕様上XDG_RUNTIME_DIRを一切参照しない（ResolveSocketPathの
// windows分岐は%LOCALAPPDATA%のみを見る）ため、この関数を使うテストは
// Windowsでは意味を持たずスキップする。
func newTestRuntimeDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("XDG_RUNTIME_DIR is not consulted on windows (see ResolveSocketPath)")
	}
	dir, err := os.MkdirTemp("/tmp", "exb")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	return dir
}

func TestListenAndDestTable_roundTrip(t *testing.T) {
	newTestRuntimeDir(t)

	mailbox := NewMailbox(4, NewLogger(&bytes.Buffer{}, false))
	l, err := Listen("nodeB")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	go Serve(l, mailbox, 1024, NewLogger(&bytes.Buffer{}, false))

	dest := NewDestTable(map[uint32]string{1: "nodeB"})
	defer dest.Close()

	dest.Send(1, []byte("hello from nodeA"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, _, outcome := mailbox.Recv(ctx, 1024)
	if outcome != RecvDelivered || string(got.Payload) != "hello from nodeA" {
		t.Fatalf("mailbox.Recv() = %q, outcome=%v", got.Payload, outcome)
	}
}

func TestListen_staleSocketIsCleanedUp(t *testing.T) {
	dir := newTestRuntimeDir(t)

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
	newTestRuntimeDir(t)

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
