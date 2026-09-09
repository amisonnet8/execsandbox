# ゲスト言語のWASI実装に起因する落とし穴

`.claude/rules/wazero-quirks.md`が「ホスト（`wazero`）が実際にどう動くか」を
扱うのに対し、本ファイルは**ゲスト側の言語ランタイムが、ホストが正しく
実装したWASI/ABIをどう扱うか**を扱う。ホストのABI境界では正しく機能している
のに、ゲスト言語の標準ライブラリ・libc実装を経由すると別の挙動に見える、
という種類の落とし穴をここに集める。

`docs/examples/`・`docs/tour/`でSDKを使った実例を書く際、実際にビルド・実行
して初めて気づく類の発見が多い。新しく気づいたらここに追記すること。

## TinyGoの`crypto/rand`は乱数拒否時のエラーを握りつぶす

**現象**: `-a/--allow`で`random`を許可しない（既定）状態でTinyGo
（`wasip1`ターゲット）で書いたゲストを実行すると、`crypto/rand.Read`は
失敗せず、**実行するたびに同じ固定値**を返す（`docs/examples/
policy-and-limits.md`・`docs/tour/14-allowing-random-and-time.md`で実測、
値は`117`）。エラーとしては一切観測できない。

**原因**: ExecSandboxのホスト側実装（`sandbox/policy.go`）は、`random`が
未許可（既定）のとき`random_get`の呼び出しを常にエラー（`sys.EIO`）で
拒否する。これは`sandbox/policy_test.go`の
`TestPolicy_random_defaultDeniesWithError`が生のABI（`testdata/modules/
wasi_probe.wasm`の`random_probe`、SDKを介さずWASIの`random_get`を直接
呼ぶ）に対して正しく機能することを確認済みであり、**ホストのABI境界では
確実に拒否できている。**

ところがTinyGoの`crypto/rand`（`wasip1`ターゲット、`src/crypto/rand/
rand_arc4random.go`）は、`random_get`を直接呼ばず、**戻り値を持たない
libc関数`arc4random_buf(void*, size_t)`を経由する**。Cの関数シグネチャに
失敗を伝える手段がないため、内部で`random_get`がエラーになっても
`arc4random_buf`はそれを呼び出し元へ伝播できない。

**教訓**: **ホストのABI境界を正しく実装しても、その先でゲスト言語の標準
ライブラリ・libcが何をするかは言語ごとに異なる。** `-a/--allow`のような
ケイパビリティ系のポリシーを検証・説明するときは、

1. **まず生のABIを直接叩く検証用モジュール**（`testdata/modules/`、
   `.claude/rules/testing.md`）でホスト側の実装が正しいことを確認し、
2. **その上でSDK経由の挙動を別途確認する**（言語ごとに結果が異なりうる
   ことを前提にする）。

の2段階を分けて考えること。1つのゲストで観測した挙動を「ExecSandboxの
仕様」として即断しないこと（ExecSandbox本体のバグではなくゲスト言語側の
制約であることが多い）。

Rust版SDK（`getrandom`クレート経由）や他言語のSDKでどう見えるかは未検証。
検証した際はここに追記すること。

## 検証時の心がけ

- `docs/examples/`・`docs/tour/`でゲストの挙動を文書化する際、**期待した
  挙動と違う結果が出たら、まずゲスト言語のランタイム実装を疑う**（ホスト側
  のバグと決めつけて`sandbox/`を疑う前に）。
- 疑わしい挙動は複数回実行して再現性を確認する（今回は4回実行して同じ
  固定値が返ることを確認してから、単なる乱数の偶然の一致ではないと判断
  した）。
