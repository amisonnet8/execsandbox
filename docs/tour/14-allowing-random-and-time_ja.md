# 14. 乱数・時刻を許可する

これまでの章はすべて「既定拒否、明示的に許可する」だった。乱数・時刻も
例外ではなく、`-a/--allow`で明示的に許可する。以下のゲストは乱数を1バイト、
現在時刻を1つ取得して表示する。

```
$ tinygo build -target=wasip1 -o allow.wasm .
$ execsandbox-build -o allow allow.wasm
$ ./allow -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
$ ./allow -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
```

既定では、時刻は2022-01-01T00:00:00Zの偽時計に固定される。乱数の方も
「エラー」にはならず、**実行するたびに同じ`117`という固定値**になる。
これは奇妙に見えるが、ExecSandbox本体のバグではない。ホストのABI境界
（`random_get`）では確実に拒否できているのだが、このゲストが使っている
TinyGoの`crypto/rand`は、WASIの`random_get`を直接呼ばず`arc4random_buf`
というlibc関数を経由する。この関数はCの慣習上、失敗を呼び出し元へ伝える
手段を持たない。結果として`random_get`のエラーは握りつぶされ、ゲスト側
からは「常に同じ値が返る」という形で観測される。

**ホストが確実に拒否していても、その先でゲストのランタイムがエラーを
どう扱うかはゲスト言語の実装次第——**というのが、この章の教訓の1つである。

`-a random,time`を指定すると、両方とも本物の値になる。

```
$ ./allow -a random,time -s out
random byte: 174
clock: 2026-09-09T00:44:49Z
$ ./allow -a random,time -s out
random byte: 128
clock: 2026-09-09T00:44:49Z
```

---
[← 前: 13. ファイルを見せる](13-exposing-files_ja.md) | [目次](README_ja.md) | [次: 15. 複数の言語で書く →](15-writing-in-another-language_ja.md)

English version: [14-allowing-random-and-time.md](14-allowing-random-and-time.md)
