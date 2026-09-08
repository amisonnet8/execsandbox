# 3. 既定では何もできない

ExecSandboxのケイパビリティは**既定拒否**である。起動時オプションで
明示しない限り、標準出力もファイルもネットワークも環境変数も、ゲストからは
何も見えない。これをコードで確認する。

以下は`fmt`と`os`だけを使う、SDKすら使わない最小のゲストである
（[`docs/examples/src/hello-wasi/main.go`](../examples/src/hello-wasi/main.go)）。

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

TinyGoでビルドし、埋め込む。

```
$ tinygo build -target=wasip1 -o hello-wasi.wasm .
$ execsandbox-build -o hello-wasi hello-wasi.wasm
```

何もオプションを付けずに実行すると、何も表示されない。

```
$ ./hello-wasi
$ echo $?
0
```

`fmt.Println`を3回呼んでいるのに、何も出ていない。ゲストはクラッシュも
していない（終了コード0）——**標準出力がどこにも接続されていない**ため、
出力は黙って捨てられている。次の章でこれを繋ぐ。

---
[← 前: 2. インストールと最初の実行](02-install-and-first-run_ja.md) | [目次](README_ja.md) | [次: 4. 出力を見る →](04-seeing-output_ja.md)

English version: [03-nothing-by-default.md](03-nothing-by-default.md)
