# 12. メモリに上限をかける

`-m/--mem-limit`は、ゲストのWASM線形メモリの上限を決める。以下のゲストは
1MiBずつスライスを確保し続け、確保できた分だけ手元に保持する（GCで回収
されて上限に達しないという事故を防ぐため）。

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

`-m 16M`でも12MiBあたりで尽きるのは、TinyGoのランタイム自体やGCの
メタデータが線形メモリの一部を既に使っているため。**ホストのプロセス
自体はクラッシュせず、ゲストの異常終了として扱われる**（終了コード1、
`execsandbox:`接頭辞のログ）——11章の`-t`と同じく、暴走したゲストが
ホスト全体を道連れにしないという保証の一種である。

---
[← 前: 11. 暴走を止める](11-stopping-a-runaway-guest.md) | [目次](README.md) | [次: 13. ファイルを見せる →](13-exposing-files.md)
