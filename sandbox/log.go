package sandbox

// ホスト側ログ（execsandbox: 接頭辞、-q/--quietによる抑制）を一手に持つ
// Logger型（.claude/rules/cli-output.md）。tail-drop・フレーム長超過は
// 複数のgoroutine（メールボックスへのPush元、各接続のserveConn）から
// 同時に呼ばれうるため、出力の混線を避けるためmutexで直列化する。

import (
	"fmt"
	"io"
	"sync"
)

type Logger struct {
	out   io.Writer
	quiet bool

	mu sync.Mutex
}

// NewLogger はoutへ書き込むLoggerを作る。quietがtrueの場合、Printfは
// 何も出力しない（-q/--quiet。仕様書§7.7、起動エラーはこの対象外であり
// main.go側で直接stderrへ書く）。
func NewLogger(out io.Writer, quiet bool) *Logger {
	return &Logger{out: out, quiet: quiet}
}

// Printf はformatをexecsandbox:接頭辞・改行付きで出力する。formatに改行を
// 含めないこと。
func (l *Logger) Printf(format string, args ...any) {
	if l.quiet {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.out, "execsandbox: "+format+"\n", args...)
}
