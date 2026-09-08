# 13. ファイルを見せる

`-v/--volume`は、ホスト側のディレクトリをゲストへマウントする。既定では
ファイルシステムへのアクセス経路そのものが存在しない。

```
$ tinygo build -target=wasip1 -o file-access.wasm .
$ execsandbox-build -o file-access file-access.wasm
$ ./file-access -s out
write failed: open /data/hello.txt: file does not exist
```

3章で見た「既定では何もできない」がファイルシステムにも一貫している。
`-v HOST:GUEST`でマウントすると、ゲスト側の`GUEST`パス（ここでは`/data`）
以下だけが見えるようになる。

```
$ mkdir hostdata
$ ./file-access -v "$PWD/hostdata:/data" -s out
write ok
read back: written by the guest
$ cat hostdata/hello.txt
written by the guest
```

`HOST:GUEST:ro`のように`:ro`を付けると読み取り専用にできる。ゲストに
見せるのはマウントしたディレクトリの中身だけであり、ホストのファイル
システム全体が見えるわけではない。

---
[← 前: 12. メモリに上限をかける](12-capping-memory.md) | [目次](README.md) | [次: 14. 乱数・時刻を遮断する →](14-denying-random-and-time.md)
