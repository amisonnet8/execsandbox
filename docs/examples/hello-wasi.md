# hello-wasi — 最小構成

ExecSandbox SDKすら使わない、最小のゲストモジュール。WASI標準の`fmt`/`os`
だけで書かれており、`-s out`・`-e`・`--`以降の引数がゲストにどう届くかを
確認できる。サンドボックス間通信や外部接続は使わない。

ソース: [`src/hello-wasi/main.go`](src/hello-wasi/main.go)

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello from execsandbox")
	fmt.Printf("args: %v\n", os.Args)
	fmt.Printf("GREETING=%s\n", os.Getenv("GREETING"))
}
```

## ビルド

TinyGoで`wasip1`ターゲット向けにビルドする。

```
$ tinygo build -target=wasip1 -o hello-wasi.wasm .
```

`execsandbox-build`で単一の実行ファイルへ埋め込む。

```
$ execsandbox-build -o hello-wasi hello-wasi.wasm
execsandbox-build: wrote hello-wasi (linux/amd64, 8788514 bytes)
```

## 実行

### `-s out`を指定しない場合

既定ではゲストの標準出力はどこにも繋がっていない（`docs/usage/
execsandbox.md`の「既定はすべて遮断」参照）。ゲスト自体は正常に実行され
終了するが、出力は見えない。

```
$ ./hello-wasi -e GREETING=konnichiwa -- foo bar
$ echo $?
0
```

### `-s out`を指定した場合

```
$ ./hello-wasi -s out -e GREETING=konnichiwa -- foo bar
hello from execsandbox
args: [execsandbox foo bar]
GREETING=konnichiwa
```

## 解説

- **`-s out`** がゲストの標準出力をホストの標準出力へ接続する。指定しない
  限り、`fmt.Println`は黙って捨てられる（`.claude/rules/cli-output.md`と
  同じ「既定拒否」の考え方が`-s`にも一貫している）。
- **`--`以降の引数**は、ゲストの`os.Args[1:]`にそのまま渡る。`os.Args[0]`は
  ゲスト自身のファイル名ではなく固定文字列`"execsandbox"`になる
  （`docs/usage/execsandbox.md`参照。埋め込まれたWASMモジュールにファイル名の
  概念がないため）。
- **`-e KEY=VALUE`** は指定した環境変数だけをゲストに見せる。ホスト側の
  環境変数がそのまま引き継がれることはない。
