package main

// 対応プラットフォーム（仕様書§6.3）と、//go:embedによる6環境分のベース
// バイナリの内包。
//
// basebinaries/配下のファイルは`make cross-base`（Makefile参照）が生成する
// ものであり、リポジトリにはコミットしない（.gitignore、
// .claude/rules/distribution.md）。したがってこのパッケージをgo build/
// go vet/go testする前には、必ず`make cross-base`を実行しておく必要がある
// （実行しないと//go:embedの対象が存在せずビルド自体に失敗する）。

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed basebinaries
var baseBinariesFS embed.FS

// target はクロスビルド対象の1プラットフォーム。
type target struct {
	GOOS, GOARCH string
}

func (t target) String() string {
	return t.GOOS + "/" + t.GOARCH
}

// filename はbasebinaries/配下に置かれた埋め込みファイル名を返す
// （Makefileの`cross-base`ターゲットが生成する命名規則と一致させること）。
func (t target) filename() string {
	name := "execsandbox_" + t.GOOS + "_" + t.GOARCH
	if t.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// supportedTargets は仕様書§6.3が定める6環境。
var supportedTargets = []target{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

// parseTarget は"GOOS/GOARCH"形式の文字列を検証し、supportedTargetsの
// いずれかに一致すればtargetを返す。
func parseTarget(s string) (target, error) {
	goos, goarch, ok := strings.Cut(s, "/")
	if !ok {
		return target{}, fmt.Errorf("invalid --target value %q, want GOOS/GOARCH (%s)", s, supportedTargetsList())
	}
	if t, ok := lookupTarget(goos, goarch); ok {
		return t, nil
	}
	return target{}, fmt.Errorf("unsupported --target value %q, want one of: %s", s, supportedTargetsList())
}

// lookupTarget はgoos/goarchの組がsupportedTargetsに含まれるか調べる。
func lookupTarget(goos, goarch string) (target, bool) {
	for _, t := range supportedTargets {
		if t.GOOS == goos && t.GOARCH == goarch {
			return t, true
		}
	}
	return target{}, false
}

func supportedTargetsList() string {
	names := make([]string, len(supportedTargets))
	for i, t := range supportedTargets {
		names[i] = t.String()
	}
	return strings.Join(names, ", ")
}

// loadBaseBinary はtに対応するベースバイナリの中身を返す。
func loadBaseBinary(t target) ([]byte, error) {
	data, err := baseBinariesFS.ReadFile("basebinaries/" + t.filename())
	if err != nil {
		return nil, fmt.Errorf("embedded base binary for %s not found: %w", t, err)
	}
	return data, nil
}
