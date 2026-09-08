# 10. イベントの種類を見分ける

前章のechoが`msg.Kind == execsandbox.KindConnData`だけを見ていたことに
気づいただろうか。`Recv`は1つの受け口に、複数の種類のイベントを合流させて
届ける。ここまでに登場したものを整理する。

| `Kind` | 意味 | `ConnID` | `Data` |
| :--- | :--- | :--- | :--- |
| `KindMessage` | サンドボックス間メッセージ（7章） | — | あり |
| `KindConnEstablished` | 外部接続が確立した | 有効 | なし |
| `KindConnData` | 外部接続からデータが届いた（9章） | 有効 | あり |
| `KindConnClosed` | 外部接続が閉じた | 有効 | なし |

前章のechoは接続の確立・切断（`KindConnEstablished`/`KindConnClosed`）を
受け取っても無視する。これはエラーではなく推奨される作法である——**知らない
/使わない`kind`はエラーにせず、無視してループを継続する。** 将来ExecSandbox
が新しい`kind`を追加しても、この作法を守っているゲストは壊れない。

`msg.ConnID`は接続を特定する。複数のクライアントが同時に接続してきても、
`ConnID`ごとに区別できる（前章のechoが受け取ったconnIDへそのまま書き戻す
だけで、複数接続を同時に処理できていたのはこのため）。

`-l`は1つのアドレスしか受け付けない点にも触れておく。複数の待ち受けが
必要なら、ExecSandboxインスタンスを複数起動し、7章のサンドボックス間
メッセージングでつなぐ。また、接続元のアドレスを取得する手段は用意されて
いない。

---
[← 前: 9. 外部から接続を受ける](09-accepting-external-connections_ja.md) | [目次](README_ja.md) | [次: 11. 暴走を止める →](11-stopping-a-runaway-guest_ja.md)

English version: [10-telling-events-apart.md](10-telling-events-apart.md)
