package sandbox

// 外部接続(仕様書§4)の受け付けと、connIDの管理。
//
// 受信方向(接続の確立・データ到着・切断)はすべてメールボックスへイベントと
// して合流させる(仕様書§4.3)。送信方向(conn_write)はConnTable.Write
// (フェーズ③Step4)が担う。

import (
	"net"
	"sync"
)

// maxReadChunk は1回のReadで読み取る最大バイト数の上限(64KiB)。
//
// ゲストはmax_frame()の値でバッファを確保する想定(仕様書§5.3「再確保の
// パスは実質的に使われない」)なので、チャンク長がそれを超えるとrecvが
// バッファ不足を返し続けて詰む。実際のチャンク長はmin(maxFrame,
// maxReadChunk)とする。64KiBという上限を別途置くのは、-fを大きく設定した
// 構成で接続ごとに巨大な読み取りバッファを常時抱えないため。
const maxReadChunk = 64 * 1024

// ConnTable は待ち受け(-l/--listen)で受け付けた外部接続の一生を管理する。
//
// connIDは1から始まる単調増加のu32とし、切断後も再利用しない(仕様書§4.2)。
// 0を使わないのは、kind=0(サンドボックス間メッセージ)の「conn_id未使用」と
// 紛らわしいため。u32が一巡した場合は生存中のconnIDを飛ばして割り当てる。
type ConnTable struct {
	mu     sync.Mutex
	nextID uint32
	conns  map[uint32]net.Conn
}

// NewConnTable は空のConnTableを作る。
func NewConnTable() *ConnTable {
	return &ConnTable{conns: make(map[uint32]net.Conn)}
}

// Serve はlからの接続を受け付け続け、確立・データ・切断の各イベントを
// mailboxへ積む。呼び出し側がgoroutineとして起動する想定で、l.Accept()が
// エラーを返すまで(典型的にはlがCloseされるまで)戻らない。
//
// 接続の受理・切断はホスト側ログに出さない(ゲストがイベントとして受け取り
// 済みの事象であり、ホストが重ねて記録する必要はないため。
// .claude/rules/cli-output.md)。
func (ct *ConnTable) Serve(l net.Listener, mb *Mailbox, maxFrame int) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		id := ct.add(conn)
		mb.Push(Message{Kind: 1, ConnID: id})
		go ct.serveConn(id, conn, mb, maxFrame)
	}
}

func (ct *ConnTable) serveConn(id uint32, conn net.Conn, mb *Mailbox, maxFrame int) {
	defer func() {
		conn.Close()
		ct.remove(id)
		// 切断イベントは仕様書§4.2により必ず配送する必要があるため、
		// メールボックス上限を無視するPushDisconnectを使う
		// (.claude/rules/testing.mdではなく本パッケージのmailbox.goの
		// PushDisconnectのコメント参照)。
		mb.PushDisconnect(id)
	}()

	chunkSize := min(maxFrame, maxReadChunk)
	if chunkSize <= 0 {
		chunkSize = maxReadChunk
	}
	buf := make([]byte, chunkSize)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			// bufは次のReadで上書きされるため、Pushする前に必ずコピーする。
			payload := append([]byte(nil), buf[:n]...)
			mb.Push(Message{Kind: 2, ConnID: id, Payload: payload})
		}
		if err != nil {
			return
		}
	}
}

func (ct *ConnTable) add(conn net.Conn) uint32 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	id := ct.allocID()
	ct.conns[id] = conn
	return id
}

// allocID はct.muを保持した状態で呼ぶこと。
func (ct *ConnTable) allocID() uint32 {
	for {
		ct.nextID++ // MaxUint32から0へ自然にラップする
		if ct.nextID == 0 {
			continue // 0は使わない(kind=0のconn_id未使用と紛らわしいため)
		}
		if _, exists := ct.conns[ct.nextID]; !exists {
			return ct.nextID
		}
	}
}

func (ct *ConnTable) remove(id uint32) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.conns, id)
}

// Close は確立済みの接続をすべて閉じる。
func (ct *ConnTable) Close() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	for id, conn := range ct.conns {
		conn.Close()
		delete(ct.conns, id)
	}
}
