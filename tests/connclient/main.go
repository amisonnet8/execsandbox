// tests/connclient は、外部接続(仕様書§4)のE2Eテスト用の最小限のクライアント。
// -l/--listenで待ち受けるサンドボックスへ接続し、指定したメッセージを送って
// 同じバイト列がそのままエコーされることを確認する。実際のビルダー・SDKとは
// 無関係なテスト補助ツールであり、配布物ではない（tests/stampと同じ位置づけ）。
//
// ncを使わないのは、windows-latestランナーにncが存在しないため
// （.claude/rules/testing.md「クロスプラットフォームCIの落とし穴」）。
// Goで書けば3OSで同じ挙動になる。
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: connclient <network> <address> <message>")
		os.Exit(2)
	}
	network, address, message := os.Args[1], os.Args[2], os.Args[3]

	conn, err := net.DialTimeout(network, address, 2*time.Second)
	if err != nil {
		// サンドボックス側の待ち受けがまだ始まっていない場合もここに来る。
		// 呼び出し側（E2Eスクリプト）がリトライする前提の終了コード1とする。
		fmt.Fprintln(os.Stderr, "connclient: dial:", err)
		os.Exit(1)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(message)); err != nil {
		fmt.Fprintln(os.Stderr, "connclient: write:", err)
		os.Exit(2)
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		fmt.Fprintln(os.Stderr, "connclient: SetReadDeadline:", err)
		os.Exit(2)
	}
	buf := make([]byte, len(message))
	if _, err := io.ReadFull(conn, buf); err != nil {
		fmt.Fprintln(os.Stderr, "connclient: read:", err)
		os.Exit(2)
	}
	if string(buf) != message {
		fmt.Fprintf(os.Stderr, "connclient: echoed %q, want %q\n", buf, message)
		os.Exit(2)
	}

	fmt.Println("OK")
}
