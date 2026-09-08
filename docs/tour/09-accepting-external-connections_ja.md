English version: [09-accepting-external-connections.md](09-accepting-external-connections.md)

# 9. 外部から接続を受ける

サンドボックス同士だけでなく、ExecSandboxの外（TCPクライアントなど）からも
つながる。`-l/--listen`で待ち受けを開く。

以下は受け取ったデータをそのまま書き戻すだけの、最小のTCPエコーサーバー
である。

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

```
$ tinygo build -target=wasip1 -o echo.wasm .
$ execsandbox-build -o echo echo.wasm
$ ./echo -l 19001 &
```

追加ツールなしで、bashの`/dev/tcp`機能から接続してみる（`nc`がない環境
でも動く）。

```
$ exec 3<>/dev/tcp/127.0.0.1/19001
$ printf 'hello via bash' >&3
$ head -c 14 <&3
hello via bash
```

送ったデータがそのまま返ってきた。ここで使っているのは`Recv`（前章までの
サンドボックス間メッセージングと**同じ受け口**）と、外部接続へ書き戻す
`ConnWrite`の2つだけである。

---
[← 前: 8. 送信は届くことを保証しない](08-delivery-is-not-guaranteed_ja.md) | [目次](README_ja.md) | [次: 10. イベントの種類を見分ける →](10-telling-events-apart_ja.md)
