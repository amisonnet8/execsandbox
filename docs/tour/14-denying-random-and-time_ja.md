English version: [14-denying-random-and-time.md](14-denying-random-and-time.md)

# 14. 乱数・時刻を遮断する

これまでの章はすべて「既定拒否、明示的に許可する」だった。`-x/--deny`は
唯一の例外——**乱数と時刻は既定で許可**されており、`-x`で明示的に
遮断する。以下のゲストは乱数を1バイト、現在時刻を1つ取得して表示する。

```
$ tinygo build -target=wasip1 -o deny.wasm .
$ execsandbox-build -o deny deny.wasm
$ ./deny -s out
random byte: 158
clock: 2026-09-08T16:02:50Z
$ ./deny -s out
random byte: 87
clock: 2026-09-08T16:02:50Z
```

`-x random,time`を指定すると、時刻は2022-01-01T00:00:00Zの偽時計に固定
される。

```
$ ./deny -x random,time -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
$ ./deny -x random,time -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
```

乱数の方は「エラー」ではなく、**実行するたびに同じ`117`という固定値**に
なる。これは奇妙に見えるが、ExecSandbox本体のバグではない。ホストの
ABI境界（`random_get`）では確実に遮断できているのだが、このゲストが
使っているTinyGoの`crypto/rand`は、WASIの`random_get`を直接呼ばず
`arc4random_buf`というlibc関数を経由する。この関数はCの慣習上、失敗を
呼び出し元へ伝える手段を持たない。結果として`random_get`のエラーは握り
つぶされ、ゲスト側からは「常に同じ値が返る」という形で観測される。

**ホストが確実に遮断していても、その先でゲストのランタイムがエラーを
どう扱うかはゲスト言語の実装次第——**というのが、この章の教訓である。

---
[← 前: 13. ファイルを見せる](13-exposing-files_ja.md) | [目次](README_ja.md) | [次: 15. 複数の言語で書く →](15-writing-in-another-language_ja.md)
