package sandbox

// -l/--listen のアドレス書式の解釈（仕様書§7.4）。

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ParseListenAddress は-l/--listenの値を net.Listen(network, address) へ
// そのまま渡せる形へ変換する。networkは"tcp"または"unix"のいずれか。
//
// GOOSに依存しない純関数にしてある（sandbox/address.goのresolveSocketPath
// と同じ考え方）。**Windowsのドライブレター付きパス（例:"C:\run\x.sock"）は、
// 先頭が"/"でもポート番号形式でもないため、"unix:"接頭辞を明示しない限り
// エラーになる**（docs/usage/execsandbox.mdに明記する）。
func ParseListenAddress(s string) (network, address string, err error) {
	const wantFormat = `want a port number, HOST:PORT, [IPv6]:PORT, a Unix socket path starting with "/", or "unix:PATH"`

	if s == "" {
		return "", "", fmt.Errorf("invalid -l/--listen value %q: empty, %s", s, wantFormat)
	}

	if rest, ok := strings.CutPrefix(s, "unix:"); ok {
		if rest == "" {
			return "", "", fmt.Errorf("invalid -l/--listen value %q: empty path after \"unix:\"", s)
		}
		return "unix", rest, nil
	}

	if strings.HasPrefix(s, "/") {
		return "unix", s, nil
	}

	// ポート番号のみ（例:"5432"）はループバックを補完する（仕様書§4.1
	// 「最も短い書き方が最も安全な設定であるべき」）。
	if isAllDigits(s) {
		if err := validatePort(s); err != nil {
			return "", "", fmt.Errorf("invalid -l/--listen value %q: %w", s, err)
		}
		return "tcp", "127.0.0.1:" + s, nil
	}

	_, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", "", fmt.Errorf("invalid -l/--listen value %q: %s", s, wantFormat)
	}
	if err := validatePort(port); err != nil {
		return "", "", fmt.Errorf("invalid -l/--listen value %q: %w", s, err)
	}
	// ホスト部が指定するのは待ち受けるインターフェースであり、接続元の制限
	// ではない（仕様書§7.4）。net.SplitHostPortは検証のみに使い、実際に
	// net.Listenへ渡すアドレスは利用者の入力をそのまま使う。
	return "tcp", s, nil
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validatePort(port string) error {
	n, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port %q, want a number", port)
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("port %d out of range (1-65535)", n)
	}
	return nil
}
