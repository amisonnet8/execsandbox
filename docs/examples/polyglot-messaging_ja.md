English version: [polyglot-messaging.md](polyglot-messaging.md)

# polyglot-messaging — TinyGoとRustの相互接続

ExecSandboxのWASM ABI（仕様書§5）は言語非依存であることを謳っている。
実際に**TinyGo版ゲストとRust版ゲストを、コードを一切変更せずに相互接続**
できることを両方向で確認する。使うのは
[`sandbox-messaging_ja.md`](sandbox-messaging_ja.md)と同じ`sender`/`receiver`の
組み合わせだが、片方をTinyGo版、もう片方をRust版に差し替える。

ソース: [`execsandbox-sdk`の`go/examples/{sender,receiver}`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/go/examples)・
[`rust/execsandbox/examples/{sender,receiver}.rs`](https://github.com/amisonnet8/execsandbox-sdk/tree/main/rust/execsandbox/examples)

## ビルド

```
$ tinygo build -target=wasip1 -o go-sender.wasm .       # go/examples/sender/
$ tinygo build -target=wasip1 -o go-receiver.wasm .     # go/examples/receiver/
$ cargo build --target wasm32-wasip1 --release --examples  # rust/execsandbox/
```

Rust側は`cargo build --examples`で`target/wasm32-wasip1/release/examples/
{sender,receiver}.wasm`が生成される。4つとも`execsandbox-build`で埋め込む。

```
$ execsandbox-build -o nodeA-go go-sender.wasm
$ execsandbox-build -o nodeB-go go-receiver.wasm
$ execsandbox-build -o nodeA-rust sender.wasm      # Rust版
$ execsandbox-build -o nodeB-rust receiver.wasm    # Rust版
```

## 実行: TinyGo送信 → Rust受信

```
$ ./nodeB-rust -n nodeB -s out &
$ ./nodeA-go -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

## 実行: Rust送信 → TinyGo受信

```
$ ./nodeB-go -n nodeB -s out &
$ ./nodeA-rust -d 1=nodeB
$ wait
kind=0 data=hello from execsandbox-sdk
```

## 解説

- **どちらの組み合わせも、同じ`nodeB`（受信側）のコードを一切変更せず
  動く。** 送信側がTinyGoかRustかを、受信側は区別できないし気にしない
  ——ExecSandboxのホストにとって両者は同じWASMモジュールでしかなく、
  「バッファはゲスト側で確保する」「メタデータは8バイト固定レイアウト」
  というABIの取り決め（`.claude/rules/abi-compatibility.md`）だけで
  相互運用性が成立している。
- TinyGo（GCあり）とRust（GCなし、バッファはスタック/ヒープを直接
  管理）という対照的なメモリモデルの2言語で成立することが、ABIが特定の
  言語ランタイムに依存していないことの実証になっている。
- 3言語目のSDKを書く場合も、`send`/`recv`/`conn_write`/`max_frame`の4関数を
  正しく`import`し、[仕様書§5](../spec/execsandbox_spec_ja.md)のレイアウトに
  従いさえすれば、この2つとそのまま相互接続できるはずである。
