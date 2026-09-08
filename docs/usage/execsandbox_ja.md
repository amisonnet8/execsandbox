English version: [execsandbox.md](execsandbox.md)

# execsandbox — 生成された実行ファイルの起動オプション

`execsandbox-build` が生成した実行ファイルの起動時オプション。WASMモジュールは
埋め込み済みのため、モジュールを指定する引数は存在しない。

```
<実行ファイル> [オプション...] [-- WASMへの引数...]
```

**本ページの内容はフェーズ④Step3完了時点（仕様書§7.1の全オプション実装済み）
の実測値。** `--help` の実際の出力は次の通り（`-V`/`-h` はビルド時の
`-ldflags -X main.version=` 埋め込み前の開発ビルドでは `dev` と表示される）。

```
$ execsandbox --help
Usage: execsandbox [options...] [-- args to the WASM module...]

Options:
  -n, --name ID                  own ID for sandbox-to-sandbox messaging
                                  (default: none, does not receive)
  -d, --dest N=ID                 assign a destination number to an ID
                                  (repeatable)
  -e, --env KEY=VALUE             environment variable (repeatable)
  -v, --volume HOST:GUEST[:ro]    mount a directory (repeatable)
                                  (default: none, no filesystem access)
  -m, --mem-limit SIZE            WASM linear memory limit (default: 512M)
  -b, --mailbox-limit N           mailbox capacity in messages (default: 1024)
  -f, --max-frame SIZE            maximum message size (default: 1M)
  -l, --listen ADDR               external connection listen address
                                  (default: none, does not listen)
  -s, --stdio LIST                streams to connect to the shell:
                                  in,out,err,all (default: none, all blocked)
  -t, --timeout DURATION          execution time limit, e.g. 30s, 5m
                                  (default: none, unlimited)
  -x, --deny LIST                 capabilities to deny: random,time
                                  (default: none, both allowed)
  -q, --quiet                     suppress host-side logging
  -h, --help                      show this help message
  -V, --version                   print the version
  -L, --print-licenses            print third-party license notices and exit

Sizes (-m, -f) accept K/M/G suffixes (1024-based), case-insensitive, with an
optional "i" (e.g. 512M, 512Mi). A number with no suffix is bytes.

Everything after "--" is passed to the WASM module as its arguments.
```

## オプション一覧

| 短 | 長 | 引数 | 内容 | 既定値 |
| :--- | :--- | :--- | :--- | :--- |
| `-n` | `--name` | `ID` | 自身のID。他のサンドボックスから宛先として指定される名前 | なし（受信しない） |
| `-d` | `--dest` | `N=ID` | 宛先番号への割り当て。繰り返し可 | なし |
| `-e` | `--env` | `KEY=VALUE` | 環境変数。繰り返し可 | なし |
| `-v` | `--volume` | `HOST:GUEST[:ro]` | ディレクトリのマウント。繰り返し可 | なし（アクセス不可） |
| `-m` | `--mem-limit` | サイズ | WASM線形メモリの上限 | `512M` |
| `-b` | `--mailbox-limit` | 整数 | メールボックスの通数上限 | `1024` |
| `-f` | `--max-frame` | サイズ | 1通の最大バイト数 | `1M` |
| `-l` | `--listen` | アドレス | 外部接続の待ち受け。1つのみ | なし（待ち受けない） |
| `-s` | `--stdio` | ストリーム列挙 | 外部に接続するストリーム | なし（すべて遮断） |
| `-t` | `--timeout` | 期間 | 実行時間制限 | なし（無期限） |
| `-x` | `--deny` | 項目列挙 | 機能の遮断 | なし（すべて許可） |
| `-q` | `--quiet` | — | ホスト側ログの抑制 | 出力する |
| `-h` | `--help` | — | ヘルプ | — |
| `-V` | `--version` | — | バージョン | — |
| `-L` | `--print-licenses` | — | 著作権表示・ライセンス全文の表示 | — |

`-v` はマウント（volume）であり、verbose ではない。バージョンは `-V`。

## 通信の設定

### `-n, --name`

自身のIDを指定する。他のサンドボックスがこのIDを `-d` で指定することで、
メッセージを送れるようになる。

省略した場合、サンドボックス間メッセージを受信しない（送信専用、あるいは
外部接続だけを受けるサンドボックスになる）。

### `-d, --dest`

宛先番号にIDを割り当てる。番号は1以上の整数。

```
-d 1=dbcore -d 2=logger
```

WASMモジュール側は番号のみを指定して送信する（`Send(1, data)`）。実際に誰と
繋がるかは、この起動オプションが決める。

割り当てられていない番号へ送信した場合、エラーにはならず黙って破棄される。

同じ番号を `-d` で2回指定するとエラーになり起動しない（後勝ちで上書きは
しない）。

## リソース制限

### `-m, --mem-limit` / `-f, --max-frame` — サイズの表記

| 記述 | 意味 |
| :--- | :--- |
| `512M` | 512 × 1024 × 1024 バイト |
| `512m` | 同上（大文字小文字を区別しない） |
| `512Mi` | 同上（`Ki`/`Mi`/`Gi` も受理する） |
| `536870912` | 単位なしはバイト数 |

`K`/`M`/`G` はいずれも1024の冪として解釈する。

**上限**: `-m` は4GiB（2<sup>32</sup>バイト。WASM線形メモリのページ数
上限に由来）、`-f` は2GiB−1バイト（ABIの`max_frame()`がi32を返すため）を
超えるとエラーになり起動しない。`-m` の指定値は64KiB（WASMの1ページ）単位に
切り上げられる（例えば`-m 1K`の実効値は64KiB）。

### `-b, --mailbox-limit`

メールボックスに溜め込める**通数**。上限に達した状態で新しいメッセージが
到着すると、そのメッセージが破棄される（tail-drop）。破棄はホスト側の標準
エラー出力に記録される。

`-b` × `-f` がメールボックス側のメモリ上界になる。既定値では
1M × 1024 = 1G。

小さいメッセージが大量に流れる構成なら、同じ上界のまま深さを増やせる。

```
-f 64K -b 16384    # 上界は同じ1G、深さは16倍
```

### `-t, --timeout`

実行時間の上限。Go標準の期間文字列（`30s`、`5m`、`1h30m`等）で指定する。
`0`以下の値はエラーになる（「タイムアウトなし」は値を指定しないことで
表す。`-t 0s`ではない）。

```
-t 30s
-t 5m
```

上限に達すると、WASMモジュールは強制終了される。プロセスの終了コードは
**124**（Unixの`timeout(1)`コマンドと同じ値）になり、ホスト側ログに
`execsandbox: execution timed out after <期間>` が出力される（`-q`で抑制可能）。

**ゲスト自身が`recv`の戻り値を確認して自発的に終了する実装になっている
場合、期限が来た時点でより穏やかに（強制終了ではなく）終了できることが
ある。** その場合の終了コードは0（あるいはゲストが指定した値）になり、
124にはならない。`recv`は期限が来るとタイムアウト相当の戻り値
（`-1`）を返すため、ゲスト側でこれを無視せず処理を終える実装にしておくと、
より予測しやすい終了になる。

## 外部接続

### `-l, --listen`

```
-l 5432
-l 127.0.0.1:5432
-l /run/mydb.sock
-l unix:/run/mydb.sock
```

| 記述 | 待ち受け |
| :--- | :--- |
| （省略） | 待ち受けない |
| `5432` | `127.0.0.1:5432`（ポートのみ指定はループバック補完） |
| `127.0.0.1:5432` | 同上 |
| `192.168.1.10:5432` | 指定インターフェースのみ |
| `:5432` | 全インターフェース |
| `[::1]:5432` | IPv6ループバック |
| `/run/mydb.sock` | Unixドメインソケット（先頭が`/`） |
| `unix:/run/mydb.sock` | 同上（`unix:`接頭辞を明示） |

ホスト部が指定するのは**待ち受けるインターフェース**であり、接続元の制限では
ない。接続元によるフィルタリングは行わないので、必要な場合はファイアウォールや
リバースプロキシを使う。

待ち受けは1インスタンスにつき1つのみ。**`-l`を2回以上指定すると起動時に
エラーになる。** 複数必要な場合は、外部接続を受けるサンドボックスを複数立てて
コアへ向ける。

**Windowsのドライブレター付きパス**（例：`C:\run\mydb.sock`）を指定する場合は、
`unix:`接頭辞が必須。先頭が`/`でもポート番号形式でもないため、接頭辞なしでは
`HOST:PORT`形式として解釈を試み、書式エラーになる。

```
-l unix:C:\run\mydb.sock
```

**Unixドメインソケットのstaleなファイルは自動的に片付けない。** `-n`による
サンドボックス間通信の待ち受け（仕様書§3.2）とは異なり、`-l`で指定するパスは
利用者が明示したものであり、勝手に削除するのは危険なため。前回のプロセスが
残したソケットファイルが残っている場合、起動時にエラーになる。事前に手動で
削除するか、`systemd`のソケットアクティベーション等、運用側で管理すること。

### 受信イベント（`recv`のkind）

`-l`で受け付けた接続は、送受信ともにメールボックスへイベントとして合流する
（仕様書§4.3、§5.3）。`recv`が返す`meta_ptr`の`kind`は以下の3種類。

| `kind` | 種別 | `conn_id` | ペイロード |
| :--- | :--- | :--- | :--- |
| `1` | 外部接続・確立 | 有効 | なし |
| `2` | 外部接続・データ | 有効 | あり |
| `3` | 外部接続・切断 | 有効 | なし |

（`kind=0`はサンドボックス間メッセージ。仕様書§5.3参照。）

`conn_id`は1から始まる単調増加の整数で、切断後も再利用しない。ゲストは
`conn_id`をキーにした状態テーブルを持ち、確立イベントで作り、データイベントで
更新し、切断イベントで破棄する。

**切断イベント（`kind=3`）は、メールボックスの通数上限（`-b`）に達していても
必ず配送される。** サンドボックス間メッセージ（`kind=0`）や確立・データ
イベントは上限到達時に破棄されうる（tail-drop）が、切断イベントだけは例外的に
上限を無視して配送する。ゲストが`conn_id`ごとの状態をいつ破棄してよいか
判断できずリークすることを防ぐための挙動である。

### `conn_write`

`conn_write(conn_id, ptr, len)`（仕様書§5.4）でゲストから接続へ書き戻す。

| 戻り値 | 意味 |
| :--- | :--- |
| `0` | 成功 |
| `-1` | 不明な`conn_id`（`-l`未指定の場合は常に`-1`）、または書き込みに失敗した場合 |

書き込みが実際に失敗した場合（相手が既に切断している等）も`-1`を返す。この
とき接続は内部的に閉じられ、切断イベント（`kind=3`）が配送されたうえで、
以後その`conn_id`は本当に「不明な`conn_id`」として扱われる。

**`--timeout`を指定している場合、`conn_write`の書き込み待ちにもその期限が
適用される。** 期限までに書き込みが完了しなければ失敗（`-1`）として扱われる。
`--timeout`を指定していない場合、書き込みは完了するまでブロックする
（相手の受信が滞っている接続への書き込みが、ゲスト全体を長時間停止させ
うる点に注意）。

## 入出力・権限

### `-s, --stdio`

外部（シェル）に接続するストリームを列挙する。カンマ区切り。

| 値 | 対象 |
| :--- | :--- |
| `in` | 標準入力 |
| `out` | 標準出力 |
| `err` | 標準エラー出力 |
| `all` | 上記すべて |

```
-s out              # 標準出力のみ
-s in,out           # 対話的なモジュール向け
-s all
```

指定しないストリームは遮断される（何も読めず、書いても捨てられる）。

ファイルパスは受け付けない。出力先の振り分けはシェルのリダイレクトで行う。

```
./mymodule -s out > output.txt
```

### `-v, --volume`

```
-v /data:/data          # 読み書き可
-v /etc/conf:/conf:ro   # 読み取り専用
```

指定しない限り、WASMモジュールはファイルシステムに一切アクセスできない。

`HOST:GUEST[:ro]` の区切りは末尾から解釈するため、Windowsのドライブレター
（`C:\data:/data` のように `HOST` 自体に `:` を含む場合）も指定できる。
`HOST` は起動時に存在確認を行い、存在しないパスを指定するとエラーで
起動しない（ディレクトリを事前に作っておく必要がある）。`GUEST` は必ず
`/` で始まる必要がある。

### `-x, --deny`

| 値 | 遮断対象 |
| :--- | :--- |
| `random` | 乱数生成 |
| `time` | 時刻取得 |

```
-x random,time
```

乱数と時刻のみ、既定で許可されている（他のポリシーは既定で禁止）。明示的に
禁止したい場合に指定する。

**`-x time`の実効的な意味について**: WASI（`clock_time_get`）には「時刻取得を
拒否する」ためのエラー経路が定義されていない。そのため`-x time`は、実際には
実行系（wazero）の既定である偽の時計（起動のたびに2022-01-01T00:00:00Z
付近から1回の呼び出しごとに1ミリ秒ずつ進むだけの、実時刻と無関係な値）を
そのまま見せる、という形で実現している。ゲストは「エラーになる」のではなく
「本物ではない値が返る」ことでしか時刻取得の禁止を知ることができない。

## その他

### `--` — WASMへの引数

`--` 以降のすべての引数を、WASMモジュールへの引数として渡す。

```
./mydb -e LOG=debug -d 1=core -- --verbose --level 3
                                 ^^^^^^^^^^^^^^^^^^ ここからWASMへ
```

### `-q, --quiet`

ExecSandbox本体が出力するログ（`execsandbox:` で始まる行）を抑制する。
WASMモジュール側の出力（`-s err`）には影響しない。

### `-L, --print-licenses`

```
./mydb -L
```

生成された実行ファイルには、ExecSandbox本体（MIT）と `wazero`
（Apache-2.0）のコードが含まれる。**この実行ファイルを第三者へ配布する
場合、両者の著作権表示とライセンス全文（wazeroはNOTICEも）を同梱する
義務が生じる。** `-L, --print-licenses` は、その義務を果たすために必要な
文面をすべて標準出力へ書き出す。`--help`/`--version` と同様、WASMモジュール
の有無に関わらず動作し、`-q` の影響も受けない。

表示義務を果たす主体を `execsandbox-build`（ビルダー）ではなく**生成された
実行ファイル自体**に持たせているのは、第三者へ配布されるのは生成物であり、
配布者がビルダーを手元に持っているとは限らないため。生成物自身が文面を
出力できれば、配布者がビルダーへのアクセスを失っていても義務を果たせる。

```
$ ./mydb -L | head -3
ExecSandbox
Licensed under the MIT License. Full text below.

```

## ホスト側ログ

本体は標準エラー出力へ `execsandbox:` を接頭辞としてログを出す。
WASMモジュール自身の出力とは区別される。**ログとエラーメッセージは英語。**

主に出力されるもの（実際の文言）：

```
execsandbox: mailbox full, dropped 3 message(s)
execsandbox: dropped 2 oversized frame(s) (max 1048576 bytes)
execsandbox: dropped 1 oversized frame(s) received from a peer (max 1048576 bytes)
execsandbox: execution timed out after 30s
```

- メールボックス上限によるメッセージ破棄（tail-drop。初回のみ即座に出力し、
  以降は一定間隔で累計数をまとめて出力する）
- `--max-frame` を超えるメッセージの破棄（送信側・受信側それぞれで検出しうる）
- `--timeout` による強制終了
- 起動時のエラー（`-q` では抑制されない。下記「起動時オプションの誤りと
  終了コード」参照）

未割り当ての宛先への送信は、ログを出さない（設定を見れば分かる静的な状態の
ため）。

`-q, --quiet` で抑制できるのは、起動時のエラーを除く上記すべて（動的に
発生しうるイベント）。WASMモジュール側の出力（`-s err`）には影響しない。

## 起動時オプションの誤りと終了コード

| 状況 | 終了コード | 出力先 |
| :--- | :--- | :--- |
| `--help` / `-h` | 0 | 標準出力 |
| `--version` / `-V` | 0 | 標準出力 |
| `--print-licenses` / `-L` | 0 | 標準出力 |
| オプションの誤り（未定義のフラグ、値の書式違反、`-v`のホストパスが存在しないなど） | 2 | 標準エラー出力（`execsandbox:`接頭辞、`-q`でも抑制されない） |
| WASMモジュールが埋め込まれていない、ホスト関数登録の失敗など（起動できなかった場合） | 1 | 標準エラー出力（`execsandbox:`接頭辞） |
| `--timeout` の期限に達し強制終了された場合 | 124 | ログは`-q`で抑制可 |
| WASMモジュールが自ら終了コードを指定した場合（例: WASI `proc_exit`） | その値をそのまま使う | — |
| WASMモジュールが正常に終了した場合 | 0 | — |

オプションエラー（終了コード2）の実際の出力例：

```
$ execsandbox -m bogus
execsandbox: invalid value "bogus" for flag -m: invalid size "bogus": must start with a number
execsandbox: run with --help for usage
```
