English version: [execsandbox-build.md](execsandbox-build.md)

# execsandbox-build — ビルダー

`.wasm` をExecSandbox本体に埋め込み、単一の実行ファイルを生成する。

```
execsandbox-build -o <出力ファイル名> [--target <GOOS>/<GOARCH>] <入力.wasm>
```

**本ページの内容はフェーズ④完了時点の実測値。** `--help` の実際の出力は
次の通り（`-V` はビルド時の `-ldflags -X main.version=` 埋め込み前の
開発ビルドでは `dev` と表示される）。

```
$ execsandbox-build --help
Usage: execsandbox-build -o <output> [--target GOOS/GOARCH] <input.wasm>

Embeds a WASM module into ExecSandbox to produce a single self-contained
executable (spec §6). The builder does not embed any sandbox policy; all
capabilities (filesystem, network, environment, ...) are chosen at the
generated executable's startup, not at build time.

Options:
  -o, --output NAME     output file name (required)
      --target GOOS/GOARCH
                         target platform (default: this machine's platform)
  -h, --help             show this help message
  -V, --version          print the version

Supported targets:
  linux/amd64     linux/arm64
  darwin/amd64    darwin/arm64
  windows/amd64   windows/arm64

If --target selects windows, ".exe" is appended to the output file name
when it isn't already present.

The generated executable embeds ExecSandbox (MIT) and wazero (Apache-2.0).
Distributing it to a third party carries their attribution obligations.
```

## インストール

**GitHub Releasesから、自分の環境に合ったビルダーをダウンロードする。**
`execsandbox-build_<タグ>_<GOOS>_<GOARCH>`（Windowsのみ `.exe`）という
名前のアセットが6環境分公開されている。ダウンロードして実行ビットを立てる
だけで使える（Goツールチェーンは不要）。

```
curl -LO https://github.com/amisonnet8/execsandbox/releases/download/<タグ>/execsandbox-build_<タグ>_linux_amd64
chmod +x execsandbox-build_<タグ>_linux_amd64
```

各アセットには `.sha256` ファイルが添付されているので、ダウンロード後に
検証できる。

```
sha256sum -c execsandbox-build_<タグ>_linux_amd64.sha256
```

**`go install` には対応していない。** ビルダーは仕様書§6.3により6環境分の
ベースバイナリを `//go:embed` で自分自身に内包する設計だが、この埋め込み
対象はリポジトリにコミットしていない（配布物をリポジトリへコミットしない
方針、`.claude/rules/distribution.md`）。そのため
`go install .../cmd/execsandbox-build@latest` を実行しても埋め込みが空の
まま（＝どのターゲットを指定してもベースバイナリが見つからずエラーになる）
ビルダーができてしまう。GitHub Releasesの6本のみを配布経路とする。

## オプション

| 短 | 長 | 引数 | 内容 | 既定値 |
| :--- | :--- | :--- | :--- | :--- |
| `-o` | `--output` | ファイル名 | 出力する実行ファイル名 | （必須） |
| — | `--target` | `GOOS/GOARCH` | 対象プラットフォーム | ビルダー自身の環境 |
| `-h` | `--help` | — | ヘルプ | — |
| `-V` | `--version` | — | バージョン | — |

`--target` に短縮形がないのは、他のオプション（`-o`/`-h`/`-V`）ほど頻用しない
ためと、`GOOS/GOARCH`という形式自体が省略しにくい情報量を持つため。

## 使い方

```
# 自分と同じ環境向け
execsandbox-build -o mydb mydb.wasm

# クロスビルド
execsandbox-build -o mydb --target linux/arm64 mydb.wasm
```

ビルダーは対応する全プラットフォーム向けのベースバイナリを内包しているため、
`--target` を変えるだけでクロスビルドできる。Goツールチェーンは不要。

## 対応プラットフォーム

```
linux/amd64     linux/arm64
darwin/amd64    darwin/arm64
windows/amd64   windows/arm64
```

`windows/*` を指定した場合、出力ファイル名の末尾が `.exe`（大文字小文字を
区別しない）でなければ自動的に付与される。

`--target` にこの6通り以外の値を指定するとエラーになり生成しない
（下記「起動時オプションの誤りと終了コード」参照）。

## 生成されるもの

```
[ベースバイナリ][WASMモジュール][フッター]
```

ベースバイナリにはExecSandbox本体と `wazero` が含まれる。生成された実行
ファイルは、起動時に自身の末尾からWASMモジュールを読み出して実行する。

ビルダーはサンドボックスポリシーを一切埋め込まない。マウント、メモリ上限、
待ち受けアドレスなどはすべて**起動時**に指定する
（[`execsandbox_ja.md`](execsandbox_ja.md)）。同じWASMモジュールから作った実行
ファイルを、異なる配線・異なる制限で何度でも起動できる。

生成された実行ファイルには実行ビット（`0o755`）が立つ。Windowsには実行
ビットの概念がないため、この設定は意味を持たない（無視される）。

## ライセンス表示について

生成された実行ファイルには、ExecSandbox本体（MIT）と `wazero`（Apache-2.0）の
コードが含まれる。**生成物を第三者へ配布する場合、両者の表示義務が生じる。**

| 対象 | ライセンス | 必要な対応 |
| :--- | :--- | :--- |
| ExecSandbox | MIT | 著作権表示とライセンス全文の同梱 |
| wazero | Apache-2.0 | 著作権表示、ライセンス全文、NOTICEの同梱 |

**この義務は、ビルダーではなく生成された実行ファイル自身が果たせる。**
生成物に `-L, --print-licenses` を渡すと、必要な文面がすべて標準出力へ
書き出される（[`execsandbox_ja.md`](execsandbox_ja.md)の「`-L, --print-licenses`」
参照）。

```
./mydb -L > THIRD-PARTY-LICENSES.txt
```

第三者へ配布されるのはビルダーではなく生成物であるため、配布者がビルダーへ
アクセスできなくても、生成物自身がこのコマンドで義務を果たせるようにして
ある。

WASMモジュール自体のライセンスは自由に選べる。どちらもコピーレフトではない
ため、生成物全体をプロプライエタリを含む任意のライセンスで配布できる。

## コード署名について

生成される実行ファイルは未署名。macOS・Windowsでは初回起動時に警告が表示される
場合がある（Gatekeeper、SmartScreen）。

これは、末尾への追記が既存の署名を壊すためである（署名はファイル全体の
ハッシュで検証される）。配布時に署名が必要な場合は、生成後に各自で署名し直す。

## 起動時オプションの誤りと終了コード

| 状況 | 終了コード | 出力先 |
| :--- | :--- | :--- |
| `--help` / `-h` | 0 | 標準出力 |
| `--version` / `-V` | 0 | 標準出力 |
| `-o/--output` 未指定、入力`.wasm`の指定漏れ・過多、`--target`の書式・値が不正 | 2 | 標準エラー出力（`execsandbox-build:`接頭辞） |
| 入力`.wasm`が読めない、出力ファイルが書けないなど（生成に失敗した場合） | 1 | 標準エラー出力（`execsandbox-build:`接頭辞） |
| 生成に成功した場合 | 0 | — |

オプションエラー（終了コード2）の実際の出力例：

```
$ execsandbox-build -o mydb
execsandbox-build: missing input .wasm file
execsandbox-build: run with --help for usage
```
