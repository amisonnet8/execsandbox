package sandbox

// 宛先番号からの遅延接続（仕様書§3.3、§3.4）。
//
// - 宛先は起動時オプション(-d N=ID)で固定される。ゲストは番号のみを扱う。
// - 接続は初回送信時に確立する。失敗した場合は次回送信時に再試行する。
// - 未割り当ての宛先、未起動・接続不可の宛先への送信は、いずれも黙って
//   捨てる（ログを出さない）。max_frame超過のログは呼び出し側
//   （ホスト関数のsend実装）の責務であり、DestTableはここに関与しない。

import (
	"net"
	"sync"
)

// DestTable は宛先番号からIDへの割り当てと、IDごとの遅延接続を保持する。
type DestTable struct {
	mu    sync.Mutex
	ids   map[uint32]string
	conns map[uint32]net.Conn
}

// NewDestTable は起動時オプション(-d)で決まった割り当て(宛先番号 -> ID)から
// DestTableを作る。assignmentsは以後変更しない前提。
func NewDestTable(assignments map[uint32]string) *DestTable {
	ids := make(map[uint32]string, len(assignments))
	for k, v := range assignments {
		ids[k] = v
	}
	return &DestTable{ids: ids, conns: make(map[uint32]net.Conn)}
}

// Send はdestへdataを送る。宛先が未割り当て、未起動、または接続不可の場合は
// 黙って何もしない。呼び出し側でmax_frameのチェックを済ませておくこと。
func (dt *DestTable) Send(dest uint32, data []byte) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	id, ok := dt.ids[dest]
	if !ok {
		return // 宛先番号が未割り当て
	}

	conn, ok := dt.conns[dest]
	if !ok {
		path, err := ResolveSocketPath(id)
		if err != nil {
			return
		}
		conn, err = net.Dial("unix", path)
		if err != nil {
			return // 宛先が未起動、または接続不可。次回送信時に再試行する。
		}
		dt.conns[dest] = conn
	}

	if err := WriteFrame(conn, data); err != nil {
		conn.Close()
		delete(dt.conns, dest)
	}
}

// Close は確立済みの接続をすべて閉じる。
func (dt *DestTable) Close() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	for dest, conn := range dt.conns {
		conn.Close()
		delete(dt.conns, dest)
	}
}
