package sandbox

// AF_UNIXでの待ち受けと、受け付けた接続からのフレーム読み取り（仕様書§3.1、
// §3.2、§4.4の待ち受け相当部分は外部接続でありここでは扱わない。あくまで
// サンドボックス間メッセージの受信側）。

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Listen はIDに対応するソケットパスで待ち受けを開始する。
//
// 既存のソケットファイルが存在する場合はまず接続を試みる。応答があれば
// 別プロセスが既にそのIDで待ち受けていると判断してエラーにする。応答が
// なければ、前回のプロセスが残した stale なファイルとみなして削除してから
// bindする（仕様書§3.2）。
func Listen(id string) (net.Listener, error) {
	path, err := ResolveSocketPath(id)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create socket directory: %w", err)
	}

	if conn, err := net.Dial("unix", path); err == nil {
		conn.Close()
		return nil, fmt.Errorf("another instance is already listening on %s", path)
	}
	// 応答がなければ前回の残骸とみなし削除する。存在しない場合のエラーは無視してよい。
	os.Remove(path)

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", path, err)
	}
	return l, nil
}

// Serve はlからの接続を受け付け続け、各接続から読み取ったフレームを
// mailboxへ積む。呼び出し側がgoroutineとして起動する想定で、l.Accept()が
// エラーを返すまで（典型的にはlがCloseされるまで）戻らない。
func Serve(l net.Listener, mailbox *Mailbox, maxFrame int, log *Logger) {
	frameLog := &rateLimitedCounter{interval: frameLogInterval}
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go serveConn(conn, mailbox, maxFrame, log, frameLog)
	}
}

func serveConn(conn net.Conn, mailbox *Mailbox, maxFrame int, log *Logger, frameLog *rateLimitedCounter) {
	defer conn.Close()
	for {
		data, oversized, err := ReadFrame(conn, maxFrame)
		if err != nil {
			return
		}
		if oversized {
			frameLog.Hit(time.Now(), func(n uint64) {
				log.Printf("dropped %d oversized frame(s) received from a peer (max %d bytes)", n, maxFrame)
			})
			continue
		}
		mailbox.Push(Message{Kind: 0, Payload: data})
	}
}
