# SDKバインディングの設計（TinyGo）

WASMモジュール側から呼ぶAPI。低レベルのWASM ABI（`execsandbox_spec_ja.md`§5）を
ラップし、Goらしい形で提供する。

> **本ドキュメントは暫定であり、本リポジトリの最終的な成果物ではない。**
>
> SDKライブラリの実装は別リポジトリ `execsandbox-sdk` で行う。同リポジトリを
> 立ち上げた時点で**本ドキュメントはそちらへ移設し、本リポジトリからは削除する**。
> それまでの間、ABIが実際にどうラップされるかの設計をここに置いている。
>
> 確定しているのは仕様書§5のABI定義のほうであり、以下のシグネチャは実装時に
> 調整されうる。

## 概要

```go
package main

import "github.com/amisonnet8/execsandbox-sdk/tinygo/execsandbox"

func main() {
    for {
        msg := execsandbox.Recv()
        switch msg.Kind {
        case execsandbox.KindMessage:
            execsandbox.Send(1, msg.Data)
        case execsandbox.KindConnOpen:
            // 接続確立
        case execsandbox.KindConnData:
            execsandbox.ConnWrite(msg.ConnID, msg.Data)
        case execsandbox.KindConnClose:
            // 状態の破棄
        }
    }
}
```

## 受信

### `Recv() Message`

メールボックスから1通取り出す。**メッセージが届くまでブロックする。**

### `RecvTimeout(d time.Duration) (Message, bool)`

タイムアウト付きで取り出す。第2戻り値が `false` ならタイムアウト。

```go
msg, ok := execsandbox.RecvTimeout(30 * time.Second)
if !ok {
    // 定期処理、ACKの諦め処理など
}
```

### `Message`

```go
type Message struct {
    Kind   Kind
    ConnID uint32   // 外部接続イベントのときのみ有効
    Data   []byte   // KindConnOpen / KindConnClose では空
}
```

### `Kind`

| 定数 | 意味 | ConnID | Data |
| :--- | :--- | :--- | :--- |
| `KindMessage` | 他のサンドボックスからのメッセージ | — | あり |
| `KindConnOpen` | 外部接続が確立した | 有効 | なし |
| `KindConnData` | 外部接続からデータが届いた | 有効 | あり |
| `KindConnClose` | 外部接続が閉じた | 有効 | なし |

**未知の `Kind` は無視してループを継続すること。** 将来イベント種別が増えた
とき、古いモジュールが壊れないようにするため。SDKの側でもそのように実装する。

## 送信

### `Send(dest int, data []byte)`

指定した宛先番号へ送る。**戻り値を持たない。**

相手に届いたかどうかを知る手段はない。宛先が未割り当て、相手が未起動、相手の
メールボックスが満杯、いずれの場合も黙って破棄される。到達確認が必要なら、
アプリケーション側でACKを設計する。

### `SendAll(data []byte)`

割り当てられているすべての宛先へ送る。`Send` を番号順に呼ぶだけのヘルパーで
あり、ホスト関数としての一斉配布は存在しない。

### `ConnWrite(connID uint32, data []byte) error`

外部接続へ書き戻す。不明な connID の場合はエラーを返す。

## その他

### `MaxFrame() int`

起動時に `-f, --max-frame` で指定された値をバイト数で返す。

SDKは起動時に一度これを呼び、内部の受信バッファをこのサイズで確保する。
そのため利用者がバッファ管理を意識する必要はない。

## 標準入出力

標準入出力はWASI標準のまま扱える。メールボックスには合流しない。

```go
fmt.Println("hello")            // -s out で有効化されていれば出力される
scanner := bufio.NewScanner(os.Stdin)  // -s in で有効化されていれば読める
```

`fmt.Println` や `os.Stdin` がそのまま使えるのは意図的な設計であり、SDKが独自の
出力関数を提供することはない。

## 制限

- **外部へ接続を開始することはできない。** 受けた接続に応答することのみ可能。
- **接続元アドレスは取得できない。**
- `Recv` の待ち口は1つに統一されている。複数のイベント源を `select` する必要は
  なく、その手段も提供しない。

## 他言語への移植

ABI仕様（`execsandbox_spec_ja.md`§5）は公開されており、他言語向けSDKは誰でも移植できる。公式には
`execsandbox-sdk` リポジトリで、TinyGoに加えて**最低1つの他言語SDK**（第一候補は
Rust）を実装し、ABIが言語非依存であることを実証する予定。GCを持たない言語で
「バッファはゲストが確保する」というABIの判断が成立することの検証になる。

移植時に守るべき点：

- インポートするモジュール名は `execsandbox`
- バッファはゲスト側で確保する。ホストのアロケータ呼び出しはない
- `recv` の戻り値の値域（0以上＝長さ、`-1`＝タイムアウト、`-2`以下＝バッファ
  不足でサイズは `-(戻り値)-1`）
- 未知の `kind` を無視する実装にすること
