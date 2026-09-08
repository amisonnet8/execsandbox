package main

import (
	"bytes"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLicenses_execsandboxMatchesRepoRoot は、go:embedで複製した
// cmd/execsandbox/licenses/execsandbox.LICENSEが、リポジトリ直下の
// LICENSEと一致することを検証する。go:embedは".."を辿れないため複製が
// 必要だが（licenses.goのコメント参照）、複製ゆえに二重管理のドリフトが
// 起こりうる。テストコードはembedの制約と異なり".."を辿れるため、ここで
// 直接比較する。
func TestLicenses_execsandboxMatchesRepoRoot(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "LICENSE"))
	if err != nil {
		t.Fatalf("read repo root LICENSE: %v", err)
	}
	if execsandboxLicenseText != string(want) {
		t.Error("cmd/execsandbox/licenses/execsandbox.LICENSE has drifted from the repo root LICENSE; copy the root LICENSE over it again")
	}
}

// TestLicenses_wazeroMatchesModuleCache は、複製したwazero.LICENSE/
// wazero.NOTICEが、go.modで指定したwazeroのバージョンの実物と一致する
// ことを、モジュールキャッシュ上のコピーと突き合わせて検証する。
// モジュールキャッシュが見当たらない場合(オフラインの開発環境等)は
// スキップする——このテストはあくまでバージョンアップ時のドリフト検知が
// 目的であり、失敗しても本質的な機能には影響しないため。
func TestLicenses_wazeroMatchesModuleCache(t *testing.T) {
	modDir, ok := findWazeroModuleCacheDir(t)
	if !ok {
		t.Skip("wazero module cache dir not found; skipping drift check")
	}

	wantLicense, err := os.ReadFile(filepath.Join(modDir, "LICENSE"))
	if err != nil {
		t.Skipf("read module cache LICENSE: %v", err)
	}
	if !bytes.Equal([]byte(wazeroLicenseText), wantLicense) {
		t.Error("cmd/execsandbox/licenses/wazero.LICENSE has drifted from the vendored wazero version; re-copy it from the module cache")
	}

	wantNotice, err := os.ReadFile(filepath.Join(modDir, "NOTICE"))
	if err != nil {
		t.Skipf("read module cache NOTICE: %v", err)
	}
	if !bytes.Equal([]byte(wazeroNoticeText), wantNotice) {
		t.Error("cmd/execsandbox/licenses/wazero.NOTICE has drifted from the vendored wazero version; re-copy it from the module cache")
	}
}

// findWazeroModuleCacheDir はgo.modに書かれたwazeroのバージョンに対応する
// モジュールキャッシュ上のディレクトリを探す。
func findWazeroModuleCacheDir(t *testing.T) (string, bool) {
	t.Helper()

	goModBytes, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		return "", false
	}
	version := ""
	for _, line := range strings.Split(string(goModBytes), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require github.com/tetratelabs/wazero ") {
			version = strings.Fields(line)[2]
			break
		}
	}
	if version == "" {
		return "", false
	}

	gomodcache := os.Getenv("GOMODCACHE")
	if gomodcache == "" {
		gomodcache = filepath.Join(build.Default.GOPATH, "pkg", "mod")
	}
	// モジュールキャッシュのディレクトリ名は大文字を"!小文字"へエスケープ
	// するが、"tetratelabs"/"wazero"はいずれも小文字のみのためそのまま
	// 結合してよい。
	dir := filepath.Join(gomodcache, "github.com", "tetratelabs", "wazero@"+version)
	if _, err := os.Stat(dir); err != nil {
		return "", false
	}
	return dir, true
}

func TestWriteLicenses_containsBothNotices(t *testing.T) {
	var buf bytes.Buffer
	writeLicenses(&buf)
	out := buf.String()

	for _, want := range []string{
		"ExecSandbox",
		"MIT License",
		"wazero",
		"Apache License",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("writeLicenses() output does not contain %q", want)
		}
	}
}
