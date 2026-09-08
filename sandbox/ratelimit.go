package sandbox

// tail-drop・フレーム長超過など「継続的に発生しうるイベント」用のログ抑制。
// 初回のみ即座に出力し、以降は一定間隔で累計数をまとめて出力する
// （.claude/rules/cli-output.md「ログのレート制限」）。

import (
	"sync"
	"time"
)

type rateLimitedCounter struct {
	interval time.Duration

	mu      sync.Mutex
	count   uint64
	lastLog time.Time
}

// Hit はイベント発生を1件記録する。初回、または前回のonFlushからinterval以上
// 経過していれば、その時点までの累計数でonFlushを呼ぶ。
func (c *rateLimitedCounter) Hit(now time.Time, onFlush func(count uint64)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count++
	if c.lastLog.IsZero() || now.Sub(c.lastLog) >= c.interval {
		n := c.count
		c.count = 0
		c.lastLog = now
		onFlush(n)
	}
}
