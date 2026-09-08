English version: [08-delivery-is-not-guaranteed.md](08-delivery-is-not-guaranteed.md)

# 8. 送信は届くことを保証しない

前の章の`nodeB`（receiver）を起動せずに、`nodeA`（sender）だけを実行して
みる。

```
$ ./nodeA -d 1=nodeB
$ echo $?
0
```

エラーは何も出ない。ただ黙って終了する——**宛先が未起動のとき、`Send`は
黙ってメッセージを破棄する。** これはバグではなく仕様である。

- 宛先が未割り当て
- 相手のサンドボックスが起動していない
- 相手のメールボックスが満杯（tail-drop）

いずれの場合も、送信側からは区別できない。`Send`は「投げっぱなし」の
Push型であり、確認応答を返す仕組みそのものが存在しない。

そのため、前章のコマンド例で`nodeB`を先に起動していたのは偶然ではない。
実運用でタイミングが保証できない場合は、届くまで送信側をリトライするか、
アプリケーション側でACKメッセージを設計する必要がある。

---
[← 前: 7. 2つのサンドボックスをつなぐ](07-connecting-two-sandboxes_ja.md) | [目次](README_ja.md) | [次: 9. 外部から接続を受ける →](09-accepting-external-connections_ja.md)
