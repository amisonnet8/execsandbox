# ExecSandbox

WASM実行ランタイムとWASMモジュールを1つの実行ファイルに封じ込める、環境構築
不要のポータブルなサンドボックス実行ツール。ケイパビリティベースのポリシーと、
Erlang風メールボックスによるメッセージングを備える。

## インストール

[Releases](https://github.com/amisonnet8/execsandbox/releases) から、
環境に合った `execsandbox-build_<タグ>_<GOOS>_<GOARCH>`
（Windowsのみ `.exe`）をダウンロードする。Goツールチェーンは不要。

```
chmod +x execsandbox-build_*
```

`.sha256` ファイルが同梱されているので、検証してから使うとよい。

```
sha256sum -c execsandbox-build_*.sha256
```

## クイックスタート

```
# WASMモジュールを単一の実行ファイルへ埋め込む
./execsandbox-build -o mydb mymodule.wasm

# 起動する。既定では何もできない（ファイルシステム・ネットワーク・
# 環境変数などは、起動時オプションで明示しない限りすべて遮断される）。
./mydb -s out -- hello
```

## ドキュメント

- [インタラクティブガイド（Gemini Notebook）](https://notebook.google.com/notebook/70022dee-dbf2-4365-af70-ad98b28a613f) —
  対話形式で調べられるノートブック
- [`docs/tour/README_ja.md`](docs/tour/README_ja.md) — 初めての人向けの
  入門ガイド。上から順に読む
- [`docs/usage/execsandbox_ja.md`](docs/usage/execsandbox_ja.md) — 生成された
  実行ファイルの起動オプション一覧
- [`docs/usage/execsandbox-build_ja.md`](docs/usage/execsandbox-build_ja.md) —
  ビルダーの使い方
- [`docs/examples/README_ja.md`](docs/examples/README_ja.md) — 実例集（動く
  コードと実際の出力）
- [`docs/spec/execsandbox_spec_ja.md`](docs/spec/execsandbox_spec_ja.md) —
  仕様書（設計判断の理由まで含む）

## ライセンス

[MIT](LICENSE)。生成される実行ファイルには [wazero](https://github.com/tetratelabs/wazero)
（Apache-2.0）のコードも含まれるため、第三者へ配布する場合は両方の表示義務が
生じる。生成物自体が `-L, --print-licenses` で必要な文面を出力できる。

---

English version: [README.md](README.md)
