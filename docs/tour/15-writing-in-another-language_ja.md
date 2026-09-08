English version: [15-writing-in-another-language.md](15-writing-in-another-language.md)

# 15. 複数の言語で書く

ExecSandboxのWASM ABI（仕様書§5）は言語非依存を謳っている。ここまで
TinyGoだけを使ってきたが、最後にRustと組み合わせてそれを確かめる。

7章と同じ`sender`/`receiver`の組み合わせで、片方をTinyGo版、もう片方を
Rust版に差し替える。

```
$ tinygo build -target=wasip1 -o go-sender.wasm .       # TinyGo版sender
$ cargo build --target wasm32-wasip1 --release --examples  # Rust版receiver
$ execsandbox-build -o nodeA-go go-sender.wasm
$ execsandbox-build -o nodeB-rust receiver.wasm
```

```
$ ./nodeB-rust -n nodeB -s out &
$ ./nodeA-go -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

逆方向（Rust送信→TinyGo受信）も同様に動く。**受信側のコードを一切変更
せずに**、送信側の言語だけを差し替えられる。ExecSandboxのホストにとって
両者は同じWASMモジュールでしかなく、「バッファはゲスト側で確保する」
「メタデータは8バイト固定レイアウト」というABIの取り決めだけで相互運用性
が成立している。

TinyGo（GCあり）とRust（GCなし）という対照的なメモリモデルの2言語で
成立していることが、ABIが特定の言語ランタイムに依存していないことの
実証になっている。3言語目のSDKを書く場合も、`send`/`recv`/`conn_write`/
`max_frame`の4関数を正しく`import`すれば、この2つとそのまま相互接続
できるはずである。

---
[← 前: 14. 乱数・時刻を遮断する](14-denying-random-and-time_ja.md) | [目次](README_ja.md) | [次: 16. 次に読むもの →](16-where-to-go-next_ja.md)
