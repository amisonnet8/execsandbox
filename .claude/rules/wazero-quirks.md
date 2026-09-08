# `wazero` 固有の落とし穴

ExecSandboxの中核である `wazero`（Pure-Go WASMランタイム）は、公式ドキュメント
だけを読んで素直に実装すると仕様（`docs/spec/execsandbox_spec_ja.md`）と
食い違う挙動になる箇所がいくつかある。ここでは実装中（主にフェーズ②）に
実際に踏んだ・確認した`wazero`固有の癖をまとめる。ExecSandbox側の設計判断は
`.claude/rules/abi-compatibility.md`等の他ファイルに譲り、ここには
「`wazero`が実際にどう動くか」だけを書く。

参照している`wazero`のバージョンは `go.mod` の記載を正とする（フェーズ②時点で
v1.12.0）。将来のバージョンアップで挙動が変わった場合はこのファイルを
更新すること。

## 乱数・時刻の既定は「サンドボックス寄り」——仕様書とは逆

ExecSandbox仕様書§8.2は、乱数と時刻を**既定で許可**としている（他のポリシーが
軒並み既定拒否なのとは対照的）。ところが`wazero`自身の既定はこの逆で、
**「本物の乱数・時刻を渡さない」方がサンドボックス的に安全**という考え方に
立っている。

| ホスト関数 | `wazero`の既定 | ExecSandboxが必要とする既定 |
| :--- | :--- | :--- |
| `ModuleConfig.WithRandSource` | 決定的な乱数（毎回同じバイト列） | 本物の乱数（`crypto/rand.Reader`） |
| `ModuleConfig.WithSysWalltime` | 偽の壁時計（後述） | 実時刻 |
| `ModuleConfig.WithSysNanotime` | 偽の単調時計（1回の呼び出しごとに1ms進むだけ） | 実際の単調時計 |
| `ModuleConfig.WithSysNanosleep` | 即座に返る（実際には眠らない） | 実際にスリープする |

**何もしなければ、`-x`を一切指定しなくても仕様と正反対（乱数は予測可能、
時刻は嘘）の状態が黙って成立する。** ExecSandboxでは`sandbox/policy.go`の
`Policy.ModuleConfig()`で、`-x random`/`-x time`が指定されていない限り
これらを明示的に呼ぶことで仕様の既定（許可）を成立させている。

`WithSysNanotime`だけを有効化して`WithSysNanosleep`を呼び忘れると、
sleepを`nanotime`のビジーループで実装している言語（Go等）のゲストで
CPUを無駄に消費し続ける。**walltime/nanotime/nanosleepは必ず三点セットで
扱うこと。**

### 偽の壁時計の具体的な値

`WithSysWalltime`を呼ばない場合の偽の壁時計は、`internal/platform/time.go`の
`FakeEpochNanos`（**2022-01-01T00:00:00Z**）を起点に、1回の呼び出しごとに
1ミリ秒ずつ加算されるだけの値になる。実装前は「0（1970年）付近の小さい値に
なるはず」と誤って想定していたが、実際にソースを確認したところ2022年基準の
一見もっともらしい大きな数値だった。**「小さい値かどうか」で判定するテストは
書けない。** `time.Now()`との差分（`time.Since`）で判定すること
（`sandbox/policy_test.go`の`TestPolicy_clock_denyKeepsTheFakeClock`参照）。

### `clock_time_get` には「拒否」のエラー経路がない

WASI（`wasi_snapshot_preview1`）の`clock_time_get`は、成功時は取得した
タイムスタンプを、失敗時はエラー番号だけを返す設計であり、「時刻取得を
拒否する」ための専用エラーコードが定義されていない。そのためExecSandboxの
`-x time`は、`clock_time_get`自体をエラーにするのではなく、**`wazero`の
既定（偽の壁時計）をそのまま見せ続ける**ことで実効的な「時刻を見せない」を
実現している（`docs/usage/execsandbox.md`にも利用者向けに明記済み）。

同様に、乱数（`random_get`）にはエラー経路があるため、`-x random`は
実際にエラー（`sys.EIO`）を返すことで遮断できる。**乱数と時刻で「拒否」の
実現方法が非対称になる**のは、ExecSandbox側の設計不備ではなくWASI側の
API設計に起因する。

`-x random`の実装では、`wazero`の決定的乱数（`WithRandSource`未設定時の
既定）をそのまま「遮断」の代わりに使わないこと。決定的でも「乱数が
取れてしまう」ことに変わりはなく、遮断のつもりが予測可能な乱数の許可に
すり替わる。常にエラーを返す`io.Reader`を明示的に渡すこと
（`sandbox/policy.go`の`alwaysErrorReader`）。

## `WithMemoryLimitPages` は既定超えでpanicする

`wazero.RuntimeConfig.WithMemoryLimitPages(pages uint32)`は、
**既定値（65536ページ＝4GiB）より大きい値を渡すとpanicする。** エラーを
返すのではなくpanicする点に注意。ExecSandboxの`-m/--mem-limit`は、
バイト数をページ数へ変換する時点（`sandbox/policy.go`の`MemoryLimitPages`）で
65536ページ超を明示的にエラーとして弾き、`WithMemoryLimitPages`へ
到達させないようにしている。

ページはWASMの仕様で64KiB固定。バイト数がページ境界に一致しない場合
（例:`-m 1K`）は、ExecSandboxでは切り上げている（詳細と理由は
`sandbox/policy.go`の`MemoryLimitPages`のコメント参照）。

`WithMemoryCapacityFromMax(true)`は、最大メモリ（宣言があればその値、
なければ`WithMemoryLimitPages`の値）を**即座に確保する**（先行確保）。
ExecSandboxは呼んでいない。上限は「超えてはいけない天井」であって
「あらかじめ確保しておく量」ではないため、既定の512Mだけで毎回512MiBを
先行確保するのは軽量なゲストに対して無駄が大きい。

## `WithCloseOnContextDone` は「強制終了の保証」であって「唯一の終了経路」ではない

`RuntimeConfig.WithCloseOnContextDone(true)`を有効にし、deadline付きの
`context.Context`を`InstantiateWithConfig`（や`Call`）へ渡すと、
deadline経過時にモジュールが強制的にクローズされ、呼び出しが
`*sys.ExitError`（`ExitCode() == sys.ExitCodeDeadlineExceeded`、値は
`0xefffffff`）で返る。これはインタプリタの実行ループが命令境界で
定期的にcontextを確認する仕組みで実現されており、**ゲストが何もチェック
していなくても最終的に止まる**という強制終了の保証を与える。

ただし、これは「唯一の停止経路」ではない。**同じcontextはホスト関数
呼び出しにもそのまま渡ってくる**ため、ホスト関数自身がcontextの
キャンセルを見て自発的に処理を終えられる場合、`WithCloseOnContextDone`が
実際に介入する前にモジュールが正常終了することがある。

ExecSandboxの`recv`（`sandbox/host.go`）はこの後者の例で、
`timeout_ms=-1`（無限待ち）でブロック中でも、渡されたcontextの`Done()`を
selectで見ており、-t由来のdeadlineが来ると自発的に`-1`（タイムアウト
相当）を返す。ゲスト側がその戻り値を律儀にチェックして終了する実装で
あれば、`exitCode 0`（あるいはゲストが指定した値）の**穏やかな終了**に
なる。逆に戻り値を無視してループし続けるゲスト（`testdata/modules/blocker.wat`
がこの例）は、`WithCloseOnContextDone`による強制終了
（`ExitCodeDeadlineExceeded`）に頼ることになる。

**この2つの経路は区別がつかない場合がある**（前者は`err == nil`、
後者は`*sys.ExitError`）ため、`-t`のE2E検証をする際は「本当に強制終了の
経路を通るゲスト」（`recv`の戻り値を無視するもの）を選ぶこと。穏やかに
終了するゲストで検証すると、`WithCloseOnContextDone`が実際に機能しているか
どうかを検証できていないテストになる（`tests/e2e_timeout.sh`が
`host_probe.wasm`ではなく`blocker.wasm`を使っているのはこのため）。

## `*sys.ExitError` はexitCode 0でも非nilの`error`になる

`sys.NewExitError(0)`は`error`インターフェースとして非nilの`*sys.ExitError`
値を返す（`ExitCode()`が0を返すだけで、Goの`err != nil`は真になる）。
ゲストがWASIの`proc_exit(0)`を呼んだ場合、`InstantiateWithConfig`等の
戻り値は「成功」を意味する`err != nil`になりうる。

**`if err != nil { return エラー扱い }`のような単純な判定をしないこと。**
`errors.As`で`*sys.ExitError`かどうかを先に判定し、`ExitCode()`の値に
応じて成功・タイムアウト・ゲスト独自の終了コードを振り分ける
（`cmd/execsandbox/main.go`の`run`参照）。

## `api.Memory.Read` は線形メモリへの生ビュー（コピーではない）

`mod.Memory().Read(offset, length)`が返す`[]byte`は、多くの場合ゲストの
線形メモリ領域を直接指すスライスであり、呼び出しのたびにコピーしている
わけではない。ホスト関数の実装で「読み取った内容を書き換える」
（`random_get`の実装が`io.ReadAtLeast`で直接書き込むように）ことは成立する。
一方、**このスライスを関数呼び出しをまたいで保持するのは危険。** メモリが
`memory.grow`で再配置される可能性があるため、あとで参照する必要がある
場合は必ずコピーすること（`[]byte`を`append([]byte(nil), ...)`等で複製する）。
ExecSandboxの`sendFunc`（`sandbox/host.go`）は、`DestTable.Send`が同期的に
フレームを書き終えることを確認した上でコピーを省略している（コメント参照）。

## WASI preopenのfd番号は3から始まる

`wasi-libc`の慣習に合わせ、`wazero`は`fd 0/1/2`をstdio、**`fd 3`以降を
`-v`等で設定したマウント（preopen）**に割り当てる
（`internal/sys/fs.go`の`FdPreopen`）。ExecSandboxのように単一のマウントしか
持たないケースで`path_open`を直接叩く検証コード（`testdata/modules/wasi_probe.wat`）
を書く場合、プリオープンのfdは決め打ちで`3`を使ってよい（複数マウントに
対応する場合は`fd_prestat_get`/`fd_prestat_dir_name`で列挙する必要がある）。
