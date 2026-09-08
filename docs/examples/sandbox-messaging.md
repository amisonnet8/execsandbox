# sandbox-messaging — サンドボックス間メッセージング

2つのExecSandboxインスタンスを、Erlang風メールボックスでつなぐ最小構成。
TinyGo版SDK（[`execsandbox-sdk`](https://github.com/amisonnet8/execsandbox-sdk)）の
`Send`/`Recv`を使う。

ソース: [`execsandbox-sdk`の`go/examples/sender`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples/sender)・
[`go/examples/receiver`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples/receiver)

```go
// sender: 宛先番号1へ1回だけ送って終了する
execsandbox.Send(1, []byte("hello from execsandbox-sdk"))
```

```go
// receiver: 1通受け取るまでブロックし、内容を表示して終了する
msg, ok := execsandbox.Recv(-1) // 負の値=無限待ち
if ok {
	fmt.Printf("kind=%d data=%s\n", msg.Kind, msg.Data)
}
```

## ビルド

```
$ tinygo build -target=wasip1 -o sender.wasm .    # sender/ で
$ tinygo build -target=wasip1 -o receiver.wasm .  # receiver/ で
$ execsandbox-build -o nodeA sender.wasm
$ execsandbox-build -o nodeB receiver.wasm
```

## 実行

`-n`で自分のIDを名乗り、`-d N=ID`で宛先番号にIDを割り当てる。receiverを
`-n nodeB`で先に起動し、senderから`-d 1=nodeB`で送る。

```
$ ./nodeB -n nodeB -s out &
$ ./nodeA -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

## 解説

- **`kind=0`は`KindMessage`**（サンドボックス間メッセージ）。外部接続の
  イベント（`KindConnEstablished`/`KindConnData`/`KindConnClosed`、
  kind=1〜3）と同じ`Recv`の戻り値で区別される（仕様書§5.3）。
- **宛先の解決は起動オプションが決める。** ゲスト側のコードは`Send(1,
  ...)`と番号だけを指定し、番号1が実際に誰を指すかは起動時の`-d 1=nodeB`が
  決める。ゲストのコードを書き換えずに宛先の組み替えができる。
- **`Send`は到達を保証しない。** senderが先に起動しreceiverがまだ`-n`で
  待ち受けを始めていないタイミングで送信すると、宛先未起動として黙って
  破棄される（仕様書§3.4）。上のコマンド例のように、届くまで送信側を
  リトライする運用が必要になる（本番のオーケストレーションではACKを
  アプリケーション側で設計する）。
