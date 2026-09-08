# 11. 暴走を止める

ここからはSDKを使わない、ExecSandbox本体のポリシー機能だけを見る4つの章
に進む。まずは`-t/--timeout`——実行時間の上限である。

以下のゲストは、`Recv`さえ呼ばない、戻り値を一切確認しない純粋な無限
ループである。

```go
func main() {
	for {
	}
}
```

```
$ tinygo build -target=wasip1 -o timeout.wasm .
$ execsandbox-build -o timeout timeout.wasm
$ time ./timeout -t 1s
execsandbox: execution timed out after 1s

real	0m1.010s
```

`-t 1s`を指定すると、ゲストが何もチェックしていなくても、1秒後に確実に
強制終了する。終了コードは`124`（Unixの`timeout(1)`コマンドと同じ慣習）。
`-q`を付けるとこのログは抑制されるが、終了コードは変わらない。

信頼できないゲストを動かすサンドボックスにとって、「ゲストが協力的で
なくても止められる」という保証は重要な性質である。

---
[← 前: 10. イベントの種類を見分ける](10-telling-events-apart.md) | [目次](README.md) | [次: 12. メモリに上限をかける →](12-capping-memory.md)
