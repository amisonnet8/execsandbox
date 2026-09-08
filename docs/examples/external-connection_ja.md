# external-connection — 外部接続をエコーバックする

ExecSandboxの外へ待ち受けを開き（`-l/--listen`）、受けたデータをそのまま
書き戻す最小のTCPエコーサーバー。TinyGo版SDKの`Recv`/`ConnWrite`を使う。

ソース: [`execsandbox-sdk`の`go/examples/echo`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples/echo)

```go
func main() {
	for {
		msg, ok := execsandbox.Recv(-1)
		if !ok {
			continue
		}
		if msg.Kind == execsandbox.KindConnData {
			execsandbox.ConnWrite(msg.ConnID, msg.Data)
		}
	}
}
```

## ビルド

```
$ tinygo build -target=wasip1 -o echo.wasm .
$ execsandbox-build -o echo echo.wasm
```

## 実行

```
$ ./echo -l 19001 &
```

接続してデータを送ると、そのまま返ってくる。追加ツールなしでbashの
`/dev/tcp`機能で試せる（`nc`が無い環境でも動く）。

```
$ exec 3<>/dev/tcp/127.0.0.1/19001
$ printf 'hello via bash' >&3
$ head -c 14 <&3
hello via bash
```

## 解説

- **`msg.Kind`で3種類のイベントを区別する。** `KindConnEstablished`
  （接続確立）・`KindConnData`（データ到着）・`KindConnClosed`（切断）が
  同じ`Recv`のループに合流してくる（サンドボックス間メッセージの
  `KindMessage`とも同じ受け口）。このechoは`KindConnData`以外を無視する
  ——仕様書§5.3が推奨する「知らない/使わない`kind`はエラーにせず無視する」
  という作法にすでに従っている。
- **`msg.ConnID`が接続を特定する。** 複数のクライアントが同時に接続しても
  connIDで区別できるが、このechoは受け取ったconnIDへ書き戻すだけなので、
  複数接続を同時に処理できる（内部で状態を持たないため）。
- **`-l`は1つのアドレスのみ受け付ける。** 複数ポートで待ち受けたい場合は
  ExecSandboxインスタンスを複数起動し、サンドボックス間メッセージング
  （[`sandbox-messaging_ja.md`](sandbox-messaging_ja.md)）で繋ぐ。
- 接続元アドレスは取得できない（仕様書§4.4の制限）。

---

English version: [external-connection.md](external-connection.md)
