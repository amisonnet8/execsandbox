# 配布方法

## 配布物は2種類

ExecDBが「実行ファイル1種類 × 6環境」だったのに対し、ExecSandboxは**利用者が
受け取るものが2つ**ある。

| 配布物 | 内容 | 環境ごとのビルド |
| :--- | :--- | :--- |
| **ビルダー**（`execsandbox-build`） | 6環境分のベースバイナリを同梱した実行ファイル | 6環境分（利用者が動かす環境の分） |
| **SDKライブラリ** | **本リポジトリの成果物ではない。** `execsandbox-sdk` リポジトリから各言語のパッケージマネージャ経由で配布される | 不要 |

**ベースバイナリ（`cmd/execsandbox` の成果物）は、単体では配布しない。**
これは利用者が直接実行するものではなく、ビルダーに内包される素材である。
素のまま実行してもWASMモジュールがないためエラーで終了する（仕様書§6.2）。

## 二段ビルドの依存関係

```
1. cmd/execsandbox を6環境分クロスコンパイル
        ↓（成果物を //go:embed の対象ディレクトリへ配置）
2. cmd/execsandbox-build を6環境分クロスコンパイル
        ↓
3. ビルダー6本をGitHub Releasesへアップロード
```

**手順1が終わらないと手順2が始められない**という依存があるため、`Makefile`
およびCIのジョブ順序に注意すること。ExecDBの `release.yml` は単純な並列
マトリクスで済んだが、こちらは段階を持つ。

なお、6×6=36通りのビルドが必要になるわけではない。手順1で作った6本を
**全ビルダーが同じように同梱する**ので、手順2も6本で足りる。

## 配布経路

| 経路 | 対象 | 内容 |
| :--- | :--- | :--- |
| GitHub Releases | 全利用者 | ビルダーの6環境分バイナリ |
| `go install .../cmd/execsandbox-build@latest` | Go開発者 | ソースからビルド。**ただし注意点あり（下記）** |

**`go install` の注意点**：ビルダーは `//go:embed` でベースバイナリを内包する
が、`go install` でソースから入れる場合、そのembed対象がリポジトリにコミット
されていなければ空になる。かといって6環境分のバイナリをコミットするのは
（下記の通り）避けたい。この矛盾をどう扱うかはフェーズ④で決めること。
考えられる案は以下。

- `go install` では「自分の環境向けのベースだけをその場でビルドして同梱する」
  形にする（クロスターゲットは使えないが、Go環境があるなら十分）
- `go install` を非対応とし、GitHub Releases一本にする

## リポジトリへのバイナリコミットは行わない

ビルド済みバイナリはリポジトリに直接コミットせず、**GitHub Releasesの
アセットとしてのみ配布する**。バイナリはGitの差分管理と相性が悪く、コミットの
たびにリポジトリサイズが際限なく増大するため。

**ExecSandboxではこれが特に重い。** ベースバイナリを6環境分コミットすると
毎回50MB以上の差分が積み上がる。

## 配布形式

ExecDBに倣い、**アーカイブ化せず生のバイナリをそのままアセットとして公開する**。
`execsandbox-build_<tag>_<goos>_<goarch>`（Windowsのみ `.exe`）。各アセットに
`sha256sum` の出力を `.sha256` ファイルとして添付する。

ダウンロードしてそのまま実行できる、というゼロセットアップの訴求と一貫させる
ための意図的な選択。

## リリースパイプライン

- **トリガー**：`v*` にマッチするタグのpush。
- **ビルド前のゲート**：`build` の前に `check`（`make check`）ジョブを挟む。
  CIを経ていないコミットへタグを打つ事故を防ぐ安全網。
- **ビルド方式**：`CGO_ENABLED=0` のクロスコンパイルを `ubuntu-latest` 1台で
  完結させる。pure Go方針のため、OSごとのランナーやCコンパイラは不要。
- **バージョン**：`git describe` ではなく、pushされたタグ名
  （`github.ref_name`）を `-ldflags -X` で渡す。タグ自体が真実の源である
  リリースビルドでは、`-dirty` サフィックス等の曖昧さがなく確実。
  **ビルダーと、そこに同梱されるベースバイナリの両方に、同じバージョンを
  埋め込むこと**（仕様書§6.3の「ビルダーのバージョン＝生成物のバージョン」
  という保証は、これによって成立する）。

## 生成物のライセンス表示

**利用者がビルダーで生成した実行ファイルには、ExecSandbox本体と `wazero` の
コードが含まれる**（仕様書§10.1）。利用者がそれを第三者へ配布する場合、
MIT（ExecSandbox）とApache-2.0（`wazero`）の表示義務が生じる。

- リリースアセットにはLICENSEを含めない方針（生バイナリ配布のため）だが、
  **README等でこの点を明示すること**。
- `--print-licenses` のようなオプションでビルダーが必要な文面を出力する案が
  仕様書§10.1にある。実装するかはフェーズ④で判断する。義務が利用者に
  あることは変わらないが、見落としを減らせる。

## SDKライブラリの配布（別リポジトリ）

SDKライブラリは `execsandbox-sdk` リポジトリで管理・配布する
（`.claude/rules/directory-structure.md`）。本リポジトリのリリース作業には
含まれない。

**本リポジトリのリリースは、SDKのリリースを待たない。** ABIは後方互換のみを保証
する前提（`.claude/rules/abi-compatibility.md`）なので、本体を先にリリースしても
既存のSDKでビルドされたWASMモジュールは動作する。逆にSDKが新しいホスト関数を
使い始める場合のみ、対応する本体のリリースが先に必要になる。

`docs/spec/sdk_binding_ja.md` は本リポジトリに置いているが、これはABIがどう
ラップされるかを示す**暫定の設計文書**であり、SDKの実装そのものではない。
`execsandbox-sdk` を立ち上げた時点でそちらへ移設し、本リポジトリからは削除する
（`.claude/rules/directory-structure.md`）。

## リポジトリのメタデータ

GitHubのDescriptionとTopicsは以下で確定している。**利用者向けドキュメントには
載せない**（利用者が読む情報ではないため）。変更する場合は提案すること。

### 本リポジトリ — Description

> Bundle a WASM module into a single self-contained binary with capability-based
> sandbox policy and Erlang-style mailbox messaging.

本プロジェクトの特徴である「単一バイナリ化」「ケイパビリティ」「メールボックス」の
3点を1文に収めた形。

### 本リポジトリ — Topics

```
execsandbox  wasm  webassembly  wazero  wasi  sandbox  actor-model
ipc  unix-socket  single-binary  go  golang  standalone  security
```

14個（GitHubの上限は20）。選定の考え方：

- **`execsandbox`は両リポジトリに付ける。** プロジェクト名で`execsandbox`と
  `execsandbox-sdk`を串刺しできる唯一のタグであり、2リポジトリ構成では重要。
- `wasm`と`webassembly`は検索者によって使う語が異なるため両方入れる。
- `wazero`は該当リポジトリ数が少なく、そこを見ている層に確実に当たる。
- **`single-binary`と`standalone`はセットで残す。** `single-binary`は個々の
  成果物の性質（1ファイルに収まる）を指し、`standalone`は「別途ランタイムの
  用意が要らない」という体験を指す。両者は矛盾しない——ExecSandboxは複数の
  インスタンスを組み合わせてシステムを構成する前提（`actor-model`・`ipc`）
  であり、「システム全体が1個」という意味には決して読ませたくない。
  当初あった`runtime`は「これ自体がランタイムである」とも読め、伝えたい
  「ランタイム不要」という体験と紛らわしいため`standalone`に差し替えた。
- `erlang`は入れない。着想元ではあるが、Erlang関連を探している層には無関係で
  ノイズになる。
- `mailbox`は入れない。GitHubのTopicとしては電子メール関連が大半を占めており、
  本プロジェクトの文脈では機能しない。メッセージングの性格は`actor-model`と
  `ipc`で示す。
- `distributed-systems`・`microservices`は、同一ホスト限定である以上は誇大となる
  ため入れない。
- **`tinygo`は本リポジトリには付けない。** TinyGoで検索する層が求めるのは
  「TinyGoから使えるライブラリ」であり、それは`execsandbox-sdk`側にある。本
  リポジトリはdevcontainerからもTinyGoを外しており（`.claude/rules/testing.md`）、
  実体を持たない。

### `execsandbox-sdk` リポジトリ（暫定）

SDKリポジトリを立ち上げる際に使う。実装する言語が確定した時点で見直すこと。

**Description**

> Language SDKs for ExecSandbox — write WASM modules that talk to each other
> through mailbox messaging.

本体側のDescriptionが単一バイナリ化とケイパビリティを打ち出しているのに対し、
こちらは**ゲスト側を書く人**が読むため視点を変えている。

**Topics**

```
execsandbox  wasm  webassembly  wasi  sdk  bindings
tinygo  rust  actor-model  ipc
```

- `tinygo`・`rust`は実装した言語。増えたら追加する。
- **`wazero`は入れない。** SDKはホスト実装を知らず、ABIしか見ていないため、
  付けると誤解を招く。
- **`sandbox`・`security`も入れない。** SDK自体は隔離を提供しない。
