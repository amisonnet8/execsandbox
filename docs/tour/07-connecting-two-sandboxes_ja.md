# 7. 2つのサンドボックスをつなぐ

ここからはTinyGo版SDK（[`execsandbox-sdk`](https://github.com/amisonnet8/execsandbox-sdk)）
の`Send`/`Recv`を使う。SDKがポインタ・長さ・バッファ確保を隠してくれるので、
ゲスト側のコードはシンプルになる。

`sender`は宛先番号1へ1回だけ送って終了する。

```go
execsandbox.Send(1, []byte("hello from execsandbox-sdk"))
```

`receiver`は1通受け取るまでブロックし、内容を表示して終了する。

```go
msg, ok := execsandbox.Recv(-1) // 負の値=無限待ち
if ok {
	fmt.Printf("kind=%d data=%s\n", msg.Kind, msg.Data)
}
```

ビルドして埋め込む。

```
$ tinygo build -target=wasip1 -o sender.wasm .    # sender/ で
$ tinygo build -target=wasip1 -o receiver.wasm .  # receiver/ で
$ execsandbox-build -o nodeA sender.wasm
$ execsandbox-build -o nodeB receiver.wasm
```

`-n`で自分のIDを名乗り、`-d N=ID`で宛先番号にIDを割り当てる。receiverを
`-n nodeB`で先に起動し、senderから`-d 1=nodeB`で送る。

```
$ ./nodeB -n nodeB -s out &
$ ./nodeA -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

`kind=0`は`KindMessage`（サンドボックス間メッセージ）を表す——のちほど
外部接続の章で、他の`kind`の値も登場する。

注目したいのは、**宛先の解決はゲストのコードではなく起動オプションが
決めている**点である。ゲスト側は`Send(1, ...)`と番号だけを指定し、番号1が
実際に誰を指すかは起動時の`-d 1=nodeB`が決める。ゲストのコードを書き換え
ずに、宛先の組み替え（`-d 1=別のサンドボックス`）ができる。

---
[← 前: 6. メールボックスという考え方](06-the-mailbox-idea_ja.md) | [目次](README_ja.md) | [次: 8. 送信は届くことを保証しない →](08-delivery-is-not-guaranteed_ja.md)

English version: [07-connecting-two-sandboxes.md](07-connecting-two-sandboxes.md)
