# 4. 出力を見る

前の章の`hello-wasi`に`-s out`（`--stdio out`）を付けて、標準出力を
ホスト側へ繋いでみる。

```
$ ./hello-wasi -s out
hello from execsandbox
args: [execsandbox]
GREETING=
```

同じゲスト、同じコードなのに、`-s out`を付けただけで出力が現れた。
`-s`はゲストのどのストリームを外部に繋ぐかを選ぶオプションで、
`in`/`out`/`err`/`all`をカンマ区切りで指定できる（既定はすべて遮断）。

`args`が`[execsandbox]`だけで、`GREETING=`が空なのはまだ引数・環境変数を
渡していないため。次の章で渡す。

## 豆知識: ホストのログとゲストの出力は別物

ExecSandbox自身も診断ログを出すことがあるが、それは`execsandbox:`という
接頭辞付きで標準エラー出力に書かれる。`-s out`で見えているのは、あくまで
**ゲストが自分で書いた**標準出力である。この2つを混同しないことが、
ログを読むときのコツになる。

---
[← 前: 3. 既定では何もできない](03-nothing-by-default_ja.md) | [目次](README_ja.md) | [次: 5. 引数と環境変数 →](05-args-and-env_ja.md)

English version: [04-seeing-output.md](04-seeing-output.md)
