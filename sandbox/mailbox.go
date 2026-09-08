package sandbox

// Mailbox は受信側プロセスが保持する上限付きのメッセージキュー（仕様書§3.6）。
//
// - 上限は通数で指定する。
// - 上限に達した状態で新規メッセージが届いた場合はtail-dropする
//   （新しいメッセージを破棄し、既存のメッセージは保持する）。
// - 破棄はゲストに通知せず、ホスト側stderrにレート制限付きで記録する。
// - 同一送信元内の順序（仕様書§3.7）を保つため、内部はFIFOで管理する。
//
// フェーズ③（外部接続）から、メールボックスはサンドボックス間メッセージ
// （kind=0）だけでなく、外部接続の確立・データ・切断（kind=1〜3、仕様書§4.3・
// §5.3）も運ぶ。Messageがそのイベント1件を表す。

import (
	"context"
	"sync"
	"time"
)

// dropLogInterval は tail-drop ログの出力間隔（初回は即時、以降はこの間隔で
// 累計数をまとめて出力する）。
const dropLogInterval = 5 * time.Second

// Message はメールボックスに積まれるイベント1件（仕様書§5.3のmeta_ptrが表す
// kind/conn_idと、ペイロードの組）。
//
// kindの値は仕様書§5.3の表に対応する。
//
//	0: サンドボックス間メッセージ（ConnID未使用、Payloadあり）
//	1: 外部接続・確立（ConnID有効、Payloadなし）
//	2: 外部接続・データ（ConnID有効、Payloadあり）
//	3: 外部接続・切断（ConnID有効、Payloadなし）
type Message struct {
	Kind    uint32
	ConnID  uint32
	Payload []byte
}

// RecvOutcome はMailbox.Recvの結果種別。ABIのrecv戻り値（仕様書§5.3）と
// 1対1に対応する。
type RecvOutcome int

const (
	// RecvDelivered はメッセージを取り出せたことを示す。メールボックスから
	// 取り除かれている。ABIの戻り値0以上（ペイロード長）に対応する。
	RecvDelivered RecvOutcome = iota
	// RecvTimedOut はctxが完了するまでメッセージが届かなかったことを示す。
	// ABIの戻り値-1に対応する。
	RecvTimedOut
	// RecvBufferTooSmall はメッセージはあるがbufCapに収まらなかったことを
	// 示す。メッセージはメールボックスに残る。ABIの戻り値-2以下
	// （-(必要サイズ)-1）に対応する。
	RecvBufferTooSmall
)

type Mailbox struct {
	limit int
	log   *Logger

	mu     sync.Mutex
	queue  []Message
	notify chan struct{}

	dropLog rateLimitedCounter
}

// NewMailbox は通数上限 limit のメールボックスを作る。tail-drop発生時のログは
// log へ書き込む。
func NewMailbox(limit int, log *Logger) *Mailbox {
	return &Mailbox{
		limit:  limit,
		log:    log,
		notify: make(chan struct{}, 1),
		dropLog: rateLimitedCounter{
			interval: dropLogInterval,
		},
	}
}

// Push はイベントをメールボックスへ追加する。上限に達している場合は
// tail-dropし、破棄したことをレート制限付きでログに記録する。
//
// 外部接続の切断イベント（kind=3）はゲストの状態リークを防ぐため必ず配送する
// 必要があり（仕様書§4.2）、こちらではなくPushDisconnectを使うこと。
func (mb *Mailbox) Push(msg Message) {
	mb.push(msg, true)
}

// PushDisconnect は外部接続の切断イベント（kind=3）を積む。
//
// 仕様書§3.6は上限到達時の一律tail-dropを定めるが、§4.2は「切断イベントは
// 必ず配送されるため、ゲストが切断時に状態を破棄していれば取り違えは起きない」
// としており、切断イベントまでtail-dropすると素直な実装ではゲストがconnIDごと
// の状態を永久に破棄できずリークする。そのためPushDisconnectは上限を無視して
// 必ず積む（tail-dropログも出さない）。上限超過分は「同時生存接続数」で上界が
// 定まるため、メールボックスのメモリ上界という§3.6の趣旨自体は損なわれない。
func (mb *Mailbox) PushDisconnect(connID uint32) {
	mb.push(Message{Kind: 3, ConnID: connID}, false)
}

func (mb *Mailbox) push(msg Message, dropOnFull bool) {
	mb.mu.Lock()
	full := dropOnFull && len(mb.queue) >= mb.limit
	if !full {
		mb.queue = append(mb.queue, msg)
	}
	mb.mu.Unlock()

	if full {
		mb.dropLog.Hit(time.Now(), func(n uint64) {
			mb.log.Printf("mailbox full, dropped %d message(s)", n)
		})
		return
	}

	select {
	case mb.notify <- struct{}{}:
	default:
	}
}

// Recv は先頭のイベントを取り出す。
//
//   - ペイロード（あれば）が bufCap に収まる場合: RecvDeliveredでそのMessageを
//     返し、メールボックスから取り除く。
//   - ペイロードが bufCap に収まらない場合: RecvBufferTooSmallで必要な
//     バイト数を返す。メッセージはメールボックスに残る（仕様書§5.3）。
//   - ctx が完了するまでイベントが届かなかった場合: RecvTimedOutを返す。
//
// ctx に締切を持たせるかどうかは呼び出し側（ホスト関数側）が
// timeout_ms の値に応じて決める。Mailbox自身はtimeout_msの意味論を知らない。
func (mb *Mailbox) Recv(ctx context.Context, bufCap int) (Message, int, RecvOutcome) {
	for {
		mb.mu.Lock()
		if len(mb.queue) > 0 {
			msg := mb.queue[0]
			if len(msg.Payload) <= bufCap {
				mb.queue = mb.queue[1:]
				mb.mu.Unlock()
				return msg, 0, RecvDelivered
			}
			mb.mu.Unlock()
			return Message{}, len(msg.Payload), RecvBufferTooSmall
		}
		mb.mu.Unlock()

		select {
		case <-mb.notify:
			continue
		case <-ctx.Done():
			return Message{}, 0, RecvTimedOut
		}
	}
}
