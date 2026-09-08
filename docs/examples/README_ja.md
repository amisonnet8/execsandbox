# 実例集

ExecSandboxで何ができるかを、動くコードと実際のコマンド出力で示す。それぞれ
自己完結しており、読む順序は問わない。オプションの一覧・書式は
[`docs/usage/`](../usage/)、設計判断の理由は[`docs/spec/`](../spec/)を参照。

| 例 | 見せるもの | 使う起動オプション |
| :--- | :--- | :--- |
| [`hello-wasi_ja.md`](hello-wasi_ja.md) | SDK不使用の最小構成 | `-s`, `-e`, `--` |
| [`sandbox-messaging_ja.md`](sandbox-messaging_ja.md) | サンドボックス間メッセージング | `-n`, `-d` |
| [`external-connection_ja.md`](external-connection_ja.md) | 外部接続のエコーバック | `-l` |
| [`policy-and-limits_ja.md`](policy-and-limits_ja.md) | ファイルアクセス・メモリ上限・タイムアウト・乱数/時刻の遮断 | `-v`, `-m`, `-t`, `-x` |
| [`polyglot-messaging_ja.md`](polyglot-messaging_ja.md) | TinyGoとRustのゲストを相互接続 | `-n`, `-d` |

`hello-wasi_ja.md`・`policy-and-limits_ja.md`のゲストソースは[`src/`](src/)に
置いている。`sandbox-messaging_ja.md`・`external-connection_ja.md`・
`polyglot-messaging_ja.md`は[`execsandbox-sdk`](https://github.com/amisonnet8/execsandbox-sdk)
リポジトリのexamplesをそのまま使う。

## 前提

いずれの例も、ゲストをビルドするツールチェーン（[TinyGo](https://tinygo.org/)
または[Rust](https://www.rust-lang.org/)のいずれか、あるいは両方）と、
`execsandbox-build`が手元にあることを前提にする。ビルダー自体の入手方法は
リポジトリ直下の[`README_ja.md`](../../README_ja.md)を参照。

---

English version: [README.md](README.md)
