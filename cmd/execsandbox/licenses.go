package main

// -L/--print-licenses（仕様書§7.1、§10.1）。
//
// 生成された実行ファイルには、ExecSandbox本体（MIT）とwazero（Apache-2.0）
// のコードが含まれる。第三者へ配布する場合、双方の著作権表示・ライセンス
// 全文の同梱義務が生じる（仕様書§10.1）。この義務を果たす主体を、ビルダー
// ではなく生成物自身に持たせる——第三者へ配布されるのは生成物であり、
// 配布者がビルダーを手元に持っているとは限らないため。
//
// go:embedは埋め込み元パスに".."を含められない（実行するパッケージの
// ディレクトリ配下しか参照できない）ため、リポジトリ直下のLICENSEを
// 直接embedすることはできない。cmd/execsandbox/licenses/配下に複製を
// 置いている（licenses_test.goが直下のLICENSEとの一致をテストで検証し、
// ドリフトを検知する）。wazero.LICENSE/wazero.NOTICEはgo.modのwazero
// バージョン（v1.12.0時点）からコピーしたもの。wazeroのバージョンを
// 上げた際は、この2ファイルも合わせて更新すること。

import (
	_ "embed"
	"fmt"
	"io"
	"strings"
)

//go:embed licenses/execsandbox.LICENSE
var execsandboxLicenseText string

//go:embed licenses/wazero.LICENSE
var wazeroLicenseText string

//go:embed licenses/wazero.NOTICE
var wazeroNoticeText string

// writeLicenses は、埋め込まれた著作権表示・ライセンス全文をwへ書き出す。
func writeLicenses(w io.Writer) {
	fmt.Fprintln(w, "ExecSandbox")
	fmt.Fprintln(w, "Licensed under the MIT License. Full text below.")
	fmt.Fprintln(w)
	fmt.Fprint(w, execsandboxLicenseText)

	fmt.Fprintln(w)
	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintln(w)

	fmt.Fprint(w, wazeroNoticeText)
	fmt.Fprintln(w, "Licensed under the Apache License, Version 2.0. Full text below.")
	fmt.Fprintln(w)
	fmt.Fprint(w, wazeroLicenseText)
}
