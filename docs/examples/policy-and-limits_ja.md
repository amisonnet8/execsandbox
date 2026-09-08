# policy-and-limits — リソース制限とケイパビリティ

`-v`（ファイルマウント）・`-m`（メモリ上限）・`-t`（タイムアウト）・`-x`
（乱数/時刻の遮断）を、それぞれ最小のTinyGoゲストで実演する。SDKは使わない
（ExecSandbox本体のポリシー機能だけを見せるため）。

ソース: [`src/policy-and-limits/`](src/policy-and-limits/)（`file-access/`・
`mem-limit/`・`timeout/`・`deny/`の4つの独立したゲスト）

## `-v, --volume` — ファイルアクセス

[`file-access/main.go`](src/policy-and-limits/file-access/main.go)は
`/data/hello.txt`（ゲスト側のマウントポイント直下）へ書き込み、読み返す。

```
$ tinygo build -target=wasip1 -o file-access.wasm .
$ execsandbox-build -o file-access file-access.wasm
```

`-v`を指定しない既定状態では、ファイルシステムへのアクセス経路自体が
存在しない。

```
$ ./file-access -s out
write failed: open /data/hello.txt: file does not exist
```

`-v HOST:/data`でホスト側ディレクトリをマウントすると書き込める。

```
$ mkdir hostdata
$ ./file-access -v "$PWD/hostdata:/data" -s out
write ok
read back: written by the guest
$ cat hostdata/hello.txt
written by the guest
```

## `-m, --mem-limit` — メモリ上限

[`mem-limit/main.go`](src/policy-and-limits/mem-limit/main.go)は1MiBずつ
スライスを確保し続け、確保できた分だけ手元に保持する（GCで回収されて
上限に達しないという事故を防ぐため）。

```
$ tinygo build -target=wasip1 -o mem-limit.wasm .
$ execsandbox-build -o mem-limit mem-limit.wasm
$ ./mem-limit -m 16M -s out
allocated 1 MiB so far
...
allocated 12 MiB so far
fatal error: out of memory
execsandbox: run WASM module: module[main] function[_start] failed: wasm error: unreachable
```

TinyGoのランタイム自体が確保領域（線形メモリ）の上限に達すると`fatal
error: out of memory`でゲストごと終了する。`-m 16M`でも12MiBあたりで
尽きるのは、TinyGoのランタイム自体やGCのメタデータが線形メモリの一部を
既に使っているため。**ホストのプロセス自体はクラッシュせず、ゲストの
異常終了として扱われる**（終了コード1、`execsandbox:`接頭辞のログ）。

## `-t, --timeout` — 実行時間制限

[`timeout/main.go`](src/policy-and-limits/timeout/main.go)は`recv`さえ
呼ばない、戻り値を一切確認しない純粋な無限ループ（`.claude/rules/
wazero-quirks.md`が言う「ホスト関数の外で完結する無限ループ」の最も単純な
形）。

```
$ tinygo build -target=wasip1 -o timeout.wasm .
$ execsandbox-build -o timeout timeout.wasm
$ time ./timeout -t 1s
execsandbox: execution timed out after 1s

real	0m1.010s
```

終了コードは`124`（Unixの`timeout(1)`コマンドと同じ慣習）。`-q`を付けると
このログは抑制されるが、終了コードは変わらない。

## `-x, --deny` — 乱数・時刻の遮断

[`deny/main.go`](src/policy-and-limits/deny/main.go)は`crypto/rand`で
1バイト、`time.Now()`で現在時刻を取得して表示する。

```
$ tinygo build -target=wasip1 -o deny.wasm .
$ execsandbox-build -o deny deny.wasm
```

既定（`-x`なし）では、乱数は実行のたびに変わり、時刻は実際の現在時刻に
一致する。

```
$ ./deny -s out
random byte: 158
clock: 2026-09-08T16:02:50Z
$ ./deny -s out
random byte: 87
clock: 2026-09-08T16:02:50Z
```

`-x random,time`を指定すると、**時刻は`docs/usage/execsandbox_ja.md`が説明する
通り2022-01-01T00:00:00Zの偽時計に固定される**が、**乱数は実行するたびに
同じ`117`という値になる**（真の乱数ではなくなるという意味では遮断されて
いるが、エラーにはならない）。

```
$ ./deny -x random,time -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
$ ./deny -x random,time -s out
random byte: 117
clock: 2022-01-01T00:00:00Z
```

### なぜ乱数は「エラー」ではなく固定値になるのか

仕様書§5.6・`.claude/rules/wazero-quirks.md`の設計では、`-x random`は
`random_get`の呼び出し自体を常にエラーで遮断する。ところが**TinyGoの
`crypto/rand`（`wasip1`ターゲット）は、WASIの`random_get`を直接呼ぶのでは
なく、`arc4random_buf`というlibc関数を経由する**。この関数はCの慣習として
戻り値を持たず、失敗を呼び出し元へ伝える手段がない。結果として、内部で
`random_get`がエラーになっても、`arc4random_buf`はそれを握りつぶし
（TinyGoのwasi-libcの実装依存で、未初期化のバッファをそのまま返すため
毎回同じ値になっていると見られる)、ゲスト側からは「常に同じ値が返る」と
いう形で観測される。

**これはExecSandbox本体のバグではなく、ゲスト言語のlibc実装に起因する
挙動である。** `-x random`はホスト側のABI境界（`random_get`）では確実に
遮断できているが、その先でゲストのランタイムがエラーをどう扱うかは
ゲスト側の実装次第、という点を示す実例になっている。Rust版SDKや生のABIを
直接叩くゲストでは異なる挙動になりうる。

---

English version: [policy-and-limits.md](policy-and-limits.md)
