package sandbox

// IDからソケットパスへの解決（仕様書§3.2）。

import (
	"fmt"
	"os"
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
	// filepath.Joinはコンパイル先OSのセパレータを使ってしまう（Windows向けに
	// ビルドしたバイナリでは、goos引数に何を渡しても"\"で結合される）。
	// ここではgoos引数の値に応じたセパレータを常に一貫させたいので、
	// path/filepathを使わず両分岐とも手組みする。これによりLinux上のテストで
	// windows分岐を検証できる（逆にWindows上でのテストでもunix分岐が
	// 期待通り"/"になる）。
	if goos == "windows" {
		if localAppData == "" {
			return "", fmt.Errorf("%%LOCALAPPDATA%% is not set")
		}
		base := strings.TrimRight(localAppData, `\/`)
		return base + `\execsandbox\` + id + `.sock`, nil
	}

	if xdgRuntimeDir != "" {
		base := strings.TrimRight(xdgRuntimeDir, "/")
		return base + "/execsandbox/" + id + ".sock", nil
	}
	return fmt.Sprintf("/tmp/execsandbox-%d/%s.sock", uid, id), nil
}
