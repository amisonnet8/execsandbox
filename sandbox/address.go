package sandbox

// IDからソケットパスへの解決（仕様書§3.2）。

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveSocketPath は自身の環境に応じて、IDに対応するソケットパスを返す。
func ResolveSocketPath(id string) (string, error) {
	return resolveSocketPath(runtime.GOOS, os.Getenv("XDG_RUNTIME_DIR"), os.Getenv("LOCALAPPDATA"), os.Getuid(), id)
}

// resolveSocketPath はResolveSocketPathの中身。GOOSや環境変数を引数で受け取り、
// 実行環境に依存せずテストできるようにしている。
func resolveSocketPath(goos, xdgRuntimeDir, localAppData string, uid int, id string) (string, error) {
	if goos == "windows" {
		if localAppData == "" {
			return "", fmt.Errorf("%%LOCALAPPDATA%% is not set")
		}
		// filepath.Joinはコンパイル先OSのセパレータ（Linux上では"/"）を使って
		// しまうため、Windows形式のパスは手組みする。これはLinux上のテストで
		// windows分岐を検証できるようにするための実装上の都合でもある。
		base := strings.TrimRight(localAppData, `\/`)
		return base + `\execsandbox\` + id + `.sock`, nil
	}

	if xdgRuntimeDir != "" {
		return filepath.Join(xdgRuntimeDir, "execsandbox", id+".sock"), nil
	}
	return filepath.Join(fmt.Sprintf("/tmp/execsandbox-%d", uid), id+".sock"), nil
}
