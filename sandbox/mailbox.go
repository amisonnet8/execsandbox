package sandbox

// Mailbox は受信側プロセスが保持する上限付きのメッセージキュー（仕様書§3.6）。
//
// - 上限は通数で指定する。
// - 上限に達した状態で新規メッセージが届いた場合はtail-dropする
//   （新しいメッセージを破棄し、既存のメッセージは保持する）。
// - 破棄はゲストに通知せず、ホスト側stderrにレート制限付きで記録する。
// - 同一送信元内の順序（仕様書§3.7）を保つため、内部はFIFOで管理する。

import (
	"context"
	"sync"
	"time"
)

// dropLogInterval は tail-drop ログの出力間隔（初回は即時、以降はこの間隔で
// 累計数をまとめて出力する）。
const dropLogInterval = 5 * time.Second

type Mailbox struct {
	limit int
	log   *Logger

	mu     sync.Mutex
	queue  [][]byte
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

// Push はメッセージをメールボックスへ追加する。上限に達している場合は
// tail-dropし、破棄したことをレート制限付きでログに記録する。
func (mb *Mailbox) Push(data []byte) {
	mb.mu.Lock()
	full := len(mb.queue) >= mb.limit
	if !full {
		mb.queue = append(mb.queue, data)
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

// Recv は先頭のメッセージを取り出す。
//
//   - メッセージが bufCap に収まる場合: そのメッセージを返し、メールボックスから
//     取り除く（requiredLen=0, timedOut=false）。
//   - メッセージはあるが bufCap に収まらない場合: data=nil, requiredLen=必要な
//     バイト数を返す。メッセージはメールボックスに残る（仕様書§5.3）。
//   - ctx が完了するまでメッセージが届かなかった場合: timedOut=true を返す。
//
// ctx に締切を持たせるかどうかは呼び出し側（ホスト関数側）が
// timeout_ms の値に応じて決める。Mailbox自身はtimeout_msの意味論を知らない。
func (mb *Mailbox) Recv(ctx context.Context, bufCap int) (data []byte, requiredLen int, timedOut bool) {
	for {
		mb.mu.Lock()
		if len(mb.queue) > 0 {
			msg := mb.queue[0]
			if len(msg) <= bufCap {
				mb.queue = mb.queue[1:]
				mb.mu.Unlock()
				return msg, 0, false
			}
			mb.mu.Unlock()
			return nil, len(msg), false
		}
		mb.mu.Unlock()

		select {
		case <-mb.notify:
			continue
		case <-ctx.Done():
			return nil, 0, true
		}
	}
}
