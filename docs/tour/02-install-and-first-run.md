# 2. インストールと最初の実行

## ビルダーを入手する

[Releases](https://github.com/amisonnet8/execsandbox/releases)から、環境に
合った`execsandbox-build_<タグ>_<GOOS>_<GOARCH>`（Windowsのみ`.exe`）を
ダウンロードする。Goツールチェーンは不要。

```
$ chmod +x execsandbox-build_*
```

`.sha256`ファイルが同梱されているので、検証してから使うとよい。

```
$ sha256sum -c execsandbox-build_*.sha256
```

## WASMモジュールを埋め込む

`execsandbox-build`は、WASMモジュールを1つの実行ファイルへ埋め込む
「ビルダー」である。手元にビルド済みの`.wasm`があれば、それを渡すだけでよい
（`mymodule.wasm`の作り方は次章以降で扱う）。

```
$ ./execsandbox-build -o mydb mymodule.wasm
execsandbox-build: wrote mydb (linux/amd64, 8788514 bytes)
```

## 実行する

```
$ ./mydb -s out -- hello
```

`mydb`はもう単体の実行ファイルであり、`execsandbox-build`もWASMランタイムも
不要になっている。`-s out`と`--`が何をしているかは、次の章から順に見ていく。

---
[← 前: 1. ExecSandboxとは](01-what-is-execsandbox.md) | [目次](README.md) | [次: 3. 既定では何もできない →](03-nothing-by-default.md)
