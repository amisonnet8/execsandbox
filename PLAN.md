# PLAN

ExecSandboxの実装計画・進捗管理ドキュメント。実装が進むにつれて随時更新すること
（特に「現在地」「保留事項」は、セッションをまたぐたびに参照・更新する）。

## 前提

仕様は `docs/spec/execsandbox_spec_ja.md` で確定済みであり、各項目を選んだ判断理由も
同書に併記されている。本ドキュメントは「何を作るか」ではなく「どの順で作るか・
今どこにいるか」を扱う。

## 開発フェーズ

実装は以下の4フェーズで進める。各フェーズとも、まずLinuxで開発し、完成後に
Windows/macOSや複数CPUアーキテクチャでの動作確認（GitHub Actions等）を行う
サイクルを繰り返す（詳細は `.claude/rules/testing.md` 参照）。

1. **①ミニマム実装（技術検証フェーズ）** — スタンプ方式・WASM ABI・サンドボックス
   間通信の3要素を、最小限の機能で薄く繋げて実装し、全体が技術的に成立するかを
   確認する。網羅性は求めない。
2. **②本体の作り込み** — ①の土台の上で、ポリシー適用・CLI・バックプレッシャー・
   ログを本格的に実装する。
3. **③外部接続** — 待ち受け、connID管理、イベントのメールボックス合流、
   `conn_write`。
4. **④ビルダーとリリース** — ベースバイナリ同梱、クロスターゲット、リリース
   パイプライン。

**フェーズ順序の意図**：外部接続（③）を後回しにしているのは、これがサンドボックス
間通信（①②）と同じメールボックスに合流する設計であり、土台が固まっていないと
イベント種別の追加が難しいため。逆にビルダー（④）を最後にしたのは、①〜③の間は
`go build` した本体に検証用WASMを手動でスタンプすれば足りるため。

**SDKライブラリは本リポジトリのスコープ外**であり、別リポジトリ
`execsandbox-sdk` で開発する（`.claude/rules/directory-structure.md`）。本体の
テストはSDKを使わず、生のABIを直接叩く検証用モジュールで行う。

`execsandbox-sdk` 側の開発は、**本フェーズ①（ABI確定・スパイク検証通過）を前提
とする**。それ以前に着手するとABIの変更で書き直しになる。同リポジトリでは、
TinyGo向けSDKに加えて**他言語のSDKを最低1つ**実装し、ABIが本当に言語非依存で
あることを実証する。第一候補はRust——GCを持たない言語で「バッファはゲストが確保
する」というABIの判断（仕様書§5.1）が成立することの検証になり、TinyGo（GCあり）
との対比が最も効くため。AssemblyScriptはGCありでTinyGoに近く対比が弱い。Zigは
言語自体の破壊的変更が続いており、失敗時にABIの問題か言語の変更かの切り分けが
難しい。**TinyGo製サンドボックスとRust製サンドボックスを相互接続するE2E**まで
到達すれば、ABIの言語非依存性が実証できる。

## フェーズ①のステップ

技術検証フェーズであるフェーズ①は、以下のステップで進める。

1. **Step 1: 足場固め＋技術検証【意思決定ゲート】**
   - `wazero` の取得、`Makefile` 作成、`.gitignore`、`LICENSE`（MIT）配置。
   - **スパイク検証**：ホスト関数のimportとゲスト線形メモリへの読み書きが、
     仕様書5章のABI（バッファはゲスト確保、ホストは書き込むのみ）で成立するかを
     実測する。特に「ゲストが確保した静的領域のアドレスをホスト関数に渡し、
     ホストがそこへ8バイトのメタデータを書き込む」経路を確認する。
   - **スパイク検証**：ゲスト側が `execsandbox` モジュールの関数をimportできる
     こと、および線形メモリのポインタ受け渡しが期待通りに動くこと。
     **まず手書きのWATを `wat2wasm` で変換して確認する**（ABIの検証として純粋で、
     ツールチェーンのコード生成の癖が混入しないため）。TinyGoでの確認は、
     検証用モジュールの管理方針が決まってから行う。
   - ここで問題が出た場合、ABI設計（仕様書5章）の見直しを提案する。
2. **Step 2: スタンプ方式の実装**
   - フッターの読み書き、`os.Executable()` からの自己読み出し。
   - ExecDBのPoC（`.claude/rules/stamp.md` 参照）から流用できる部分と、
     不要な部分の切り分けをここで確定させる。
   - この時点ではビルダーは作らず、`cat base wasm footer > out` 相当の
     シェルスクリプトかGoの小さなツールで代用してよい。
3. **Step 3: 最小の実行経路**
   - 埋め込まれたWASMを取り出し、`wazero` で実行する。
   - ホスト関数 `send`/`recv`/`max_frame` を登録（`conn_write` はフェーズ③）。
   - メールボックス（上限付きキュー、tail-drop）の実装。
4. **Step 4: サンドボックス間通信**
   - AF_UNIXでの待ち受けと接続、フレーミング（長さプレフィックス）。
   - `-n`/`-d` によるID解決とケイパビリティ。
   - 遅延接続（初回`send`時に確立、失敗時は次回再試行）。
5. **Step 5: 疎通確認とCI**
   - 2つのサンドボックスを起動し、片方から他方へメッセージが届くことを確認する
     E2Eスクリプト。
   - GitHub Actions 3OSマトリクス。**特にWindowsでのAF_UNIXは早期に確認する**
     （仕様書3.1でAF_UNIX一本化を決めた前提が崩れると設計に戻るため）。

**フェーズ①の意図的なスコープ外（フェーズ②〜④で拾う）**：外部接続一式
（`-l`、connID、`conn_write`）、`-v`/`-m`/`-t`/`-x` などポリシー系オプション、
`recv` のタイムアウト（Step 3では無限待ちのみ）、ビルダー、クロスターゲット。

**フェーズ①完了の判定**：
- スタンプした実行ファイルが、埋め込まれたWASMを実行できる。 ✅（Step2/3）
- 2つのサンドボックス間で `send`/`recv` が機能する。 ✅（Step4で実プロセス2つで
  確認、Step5で`tests/e2e_basic.sh`として自動化）
- メールボックス上限に達したときtail-dropが起き、ホスト側stderrに記録される。
  ✅（`sandbox/mailbox_test.go`の`TestMailbox_tailDrop`、Step3から存在）
- GitHub Actions 3OSマトリクスがgreen（**WindowsでのAF_UNIX疎通を含む**）。
  ✅ 初回pushでmacOS/Windowsが2種類のバグ（パス区切り、AF_UNIXの
  `sun_path`長）でFAILしたが、修正して再pushしたところ全ジョブgreenを
  確認した。

**フェーズ①（ミニマム実装）は完了した。** 次はフェーズ②
（本体の作り込み：ポリシー適用・CLI・バックプレッシャー・ログ）に進む。

## フェーズ②のステップ

本体の作り込みであるフェーズ②は、以下の8ステップで進める。`-l/--listen`
（外部接続）はフェーズ③スコープのため今回は実装しない。

**最重要の発見**：wazeroの乱数・時刻の既定値は「サンドボックス寄り」（偽の
時刻、決定的な乱数）だが、ExecSandboxの仕様（§8）は逆に「乱数・時刻は既定で
許可」としている。何もしなければ仕様と正反対の状態が黙って動くため、Step 6で
専用に扱う。

**事前確認済みの設計判断**：
- `-t`の期間書式はGo標準の`time.ParseDuration`互換文字列（`"30s"`、`"5m"`等）。
- WASI経由のポリシー検証用テストモジュールは、引き続き手書きWATで作成する
  （TinyGoは導入しない）。
- `-q`は起動エラーを抑制しない。`--`以降の引数にはargv[0]相当を補う。
  終了コード体系は仕様書を変更せず実装コメントに留める。

1. **Step 1: `-t`中断機構のスパイク検証【意思決定ゲート】**
   - PLAN.md保留事項「`recv`でブロック中のゲストを、`-t`の強制終了時に
     どう安全に中断するか」を最初に実測で確定させる。
   - `testdata/modules/blocker.wat`（`_start`が`recv(timeout_ms=-1)`を
     呼び続ける、ホスト関数内ブロックの経路）と`spin_forever.wat`
     （`(loop (br 0))`のみ、純WASMループの経路）を用意し、
     `wazero.NewRuntimeConfig().WithCloseOnContextDone(true)`＋
     `context.WithTimeout`で両経路とも期限内に`*sys.ExitError`
     （`ExitCodeDeadlineExceeded`）で終了することを確認する。
   - **ゲート**：不成立ならHostConfigに中断用チャネルを足すプランBへ切り替え、
     その旨をここに記録して報告する。
2. **Step 2: CLI足場（全オプションのパースとヘルプ/バージョン）**
   - `cmd/execsandbox/options.go`を新設し`parseArgs`をmainから切り出す。
   - `-e`/`-v`/`-s`/`-x`/`-m`/`-f`/`-t`用の`flag.Value`実装、§7.3準拠の
     サイズパーサ、手書きの`writeUsage`。
   - `-h`/`-V`（stdoutへexit 0）、パースエラーは`execsandbox:`接頭辞＋
     英語でstderrへexit 2。`-d`の重複番号指定はエラーにする。
3. **Step 3: ログの統一とバックプレッシャー（`-q`/`-b`/`-f`）の実配線**
   - `sandbox/log.go`の`Logger`型に`execsandbox:`接頭辞・`-q`抑制を集約し、
     `mailbox.go`/`host.go`/`listener.go`を置き換える。
   - `-b`/`-f`をハードコード定数から実際のオプション値へ配線。`-f`は
     `math.MaxInt32`以下に制限（ABIの`max_frame()`がi32のため）。
4. **Step 4: WASI組み込みとModuleConfig土台（`-s`/`-e`/`--`引数）**
   - `wasi_snapshot_preview1.Instantiate`を追加（フェーズ①では未登録）。
   - `sandbox/policy.go`を新設し、`-s`のstdio配線、`--`以降の
     `WithArgs("execsandbox", ...)`を実装。
   - `testdata/modules/wasi_probe.wat`（環境変数・引数をstdoutへ書き出す）で
     検証。**このステップ完了時に既存のE2E・単体テストを全て再実行し、
     WASI登録が既存モジュールへ影響しないことを確認する。**
5. **Step 5: ファイルシステム（`-v`）とメモリ上限（`-m`）**
   - `-v`の`HOST:GUEST[:ro]`パースはWindowsドライブレターと衝突しないよう
     右から解釈し、GOOS非依存の純関数にする（`address.go`の
     `resolveSocketPath(goos, ...)`と同じ手口）。
   - `-m`はバイト→ページ変換し65536ページ（4GiB）超はエラー（超えると
     `WithMemoryLimitPages`がpanicするため）。`WithMemoryCapacityFromMax`は
     呼ばない（先行確保を避ける）。
   - `testdata/modules/mem_hog.wat`（`memory.grow`の失敗をもって上限を確認）。
6. **Step 6: 乱数・時刻（`-x/--deny`）【wazeroの既定が仕様と逆転する箇所】**
   - `-x`未指定時は明示的に`WithRandSource(crypto/rand.Reader)`＋
     `WithSysWalltime()`＋`WithSysNanotime()`＋`WithSysNanosleep()`を呼ぶ
     （nanotimeだけ有効化するとsleepがビジーループになる）。
   - `-x random`は常にエラーを返す`io.Reader`（wazeroの決定的乱数を
     流用しない）。`-x time`はWASIの`clock_time_get`にエラー経路がないため
     「取得を遮断」ではなく「wazeroの既定＝偽の単調時計のまま」が実効的な
     意味になる（`docs/usage`に明記する）。
   - **回帰防止の要**：既定（`-x`なし）で`clock_time_get`が`time.Now()`と
     数秒以内に一致すること、`random_get`が実行のたびに異なるバイト列を
     返すことを自動テストで固定する。
7. **Step 7: `-t/--timeout`の本実装と終了コード整理**
   - Step 1で確定した`WithCloseOnContextDone`を`-t`指定時のみ有効化し、
     ゲスト実行にのみ`context.WithTimeout`を適用する。
   - `*sys.ExitError`を`errors.As`で判定し、`ExitCodeDeadlineExceeded`は
     専用ログ＋専用終了コード、それ以外はゲストの終了コードをそのまま
     プロセスの終了コードにする。
   - 本Stepの完了をもって、保留事項「recvのタイムアウト実装方式」を解決済みに
     更新する。
8. **Step 8: E2E拡張・ドキュメント追随・仕上げ**
   - `tests/e2e_policy.sh`・`tests/e2e_timeout.sh`を新設し
     `.github/workflows/test.yml`に追加。Windows(Git Bash/MSYS)のパス変換の
     罠に対処。
   - `docs/usage/execsandbox.md`を実測値へ差し替え（`docs/spec/`は変更しない）。
   - **`.claude/rules/wazero-quirks.md`の新設を提案する**（walltime/nanotime/
     randの既定逆転、`WithMemoryLimitPages`のpanic条件等を収録）。
   - 「現在地」をフェーズ②完了へ更新し、完了判定リストを記載する。

**検証方針（共通）**：各Stepとも`go build ./...`→`go vet ./...`→
`gofmt -l .`→単体テスト→実際にビルド・スタンプした実行ファイルでの目視確認、
を最小単位とする。`make check`/`make race`/`make test`をStep 4完了時・
Step 8完了時に通す。コミットは各Step完了時に行う。pushは行わない
（GitHub操作はユーザーが行う）。

## 現在地

**フェーズ②/ Step 2 完了 → Step 3（未着手）**

Step 2（CLI足場：全オプションのパースとヘルプ/バージョン）を完了した。

- `cmd/execsandbox/options.go`（新設）: `options`構造体と`parseArgs`。
  仕様書§7.1の全オプション（`-n`/`-d`/`-e`/`-v`/`-m`/`-b`/`-f`/`-s`/`-t`/
  `-x`/`-q`/`-h`/`-V`）を登録。`-m`/`-f`共通の`parseSize`（§7.3のK/M/G・
  Ki/Mi/Gi・大文字小文字非区別）、`-t`は`time.ParseDuration`（確定済み
  方針）、`-s`/`-x`はカンマ区切り集合、`-v`は`HOST:GUEST[:ro]`を
  **右から**解釈しWindowsドライブレターと衝突しないようにした（GOOS非依存の
  純関数、`sandbox/address.go`の`resolveSocketPath`と同じ手口）。`-d`は
  重複する宛先番号をエラーにするよう変更した。
- `cmd/execsandbox/usage.go`（新設）: 手書きのヘルプ文言（`fs.PrintDefaults()`
  は短形・長形が別エントリになり表形式にできないため使わない）。
- `cmd/execsandbox/main.go`: `-h`/`-V`はstdoutへexit 0。パースエラーは
  `execsandbox:`接頭辞＋英語でstderrへexit 2（フェーズ①で許容していた
  「標準flagパッケージの出力のまま」を解消した）。`var version = "dev"`を
  追加（ldflagsでの上書きはフェーズ④）。`-b`/`-f`は`options`の値を
  実際に`NewMailbox`/`HostConfig.MaxFrame`へ配線した（Step3で計画していた
  「-b/-fの配線」のうちこの部分は`options`構造体が既に正しい既定値を
  持つため前倒しで完了。Step3の残りはログの統一（`-q`）と`-f`の
  ABI上限（`math.MaxInt32`）チェック）。
- `-e`/`-v`/`-s`/`-t`/`-x`は**パース・検証のみ**で、wazeroへの配線は
  未実装（構造体フィールドとして保持するだけ）。フェーズ②の以降のステップ
  （WASI組み込み・ファイルシステム・乱数時刻・タイムアウト）で配線する。
- `-l/--listen`は仕様書に存在するが未定義のまま（フェーズ③スコープ）。
- `cmd/execsandbox/options_test.go`（新設）: `parseSize`・`envList`・
  `parseVolume`（Windows形式含む）・`stdioSet`・`denySet`・
  `durationValue`・`destAssignments`の重複検出・`--`以降の分離・
  ヘルプ/バージョン検出・既定値のテーブルテスト。
- **実際の動作確認**: ビルドした実行ファイルで`--help`・`-V`・`-m bogus`・
  `-s foo`・`-x bar`・`-v /a`・重複`-d`・`-t 0s`・スタンプなし実行を
  すべて実行し、期待通りの終了コード・英語メッセージ・
  `execsandbox:`接頭辞を確認した。`make check`・`make race`・`make test`
  すべてgreen。

Step 1（`-t`中断機構のスパイク検証）を完了した。**ゲート合格、プランB不要。**

- `testdata/modules/blocker.wat`（`_start`が`recv(timeout_ms=-1)`を無限に
  呼び続ける、ホスト関数内ブロックの経路）と`spin_forever.wat`
  （`(loop (br 0))`のみ、純WASMループの経路）を追加。
- `sandbox/timeout_spike_test.go`で、`wazero.NewRuntimeConfig().
  WithCloseOnContextDone(true)`＋`context.WithTimeout(200ms)`の組み合わせで
  両経路とも期限通り（実測200ms前後）に`*sys.ExitError`
  （`ExitCode() == sys.ExitCodeDeadlineExceeded`）で終了することを確認した。
  `start.Call(ctx)`は`InstantiateModule`に渡したctxをそのまま使う
  （`runtime.go`で確認済み）ため、`InstantiateWithConfig`に
  `context.WithTimeout`由来のctxを渡すだけで`_start`の自動実行にも効く。
- これにより、Step 7で`-t`を実装する際は`RuntimeConfig.
  WithCloseOnContextDone(opts.timeout > 0)`＋ゲスト実行にのみ
  `context.WithTimeout(ctx, opts.timeout)`を適用する方針で進めてよいことが
  確定した。`sandbox/host.go`の`recvFunc`側の追加対応（中断用チャネル等）は
  不要。
- `make check`・`make race`で既存テストに影響がないことを確認済み。

Step 5（疎通確認とCI）を実装しpushしたところ、`test(macos-latest)`・
`test(windows-latest)`・`race(macos-latest)`がFAILした。いずれも
ExecSandbox自身の実装バグであり、以下の通り修正済み（詳細と教訓は
`.claude/rules/testing.md`「クロスプラットフォームCIの落とし穴」に追記した）。

1. **Windows: `TestResolveSocketPath`のunix分岐が失敗** —
   `sandbox/address.go`の`resolveSocketPath`が、windows分岐だけ
   `path/filepath`を避けて手組みにしていたが、**unix分岐（XDG_RUNTIME_DIR・
   /tmpフォールバック）では`filepath.Join`を使ったままだった**。
   `filepath.Join`はコンパイル先OS（この場合Windows）のセパレータを使う
   ため、テストがgoos引数に"linux"を渡していても実際には`\`区切りになり
   期待値と不一致になった。→ unix分岐も`filepath.Join`をやめ文字列結合に
   統一した。
2. **macOS: `TestListenAndDestTable_roundTrip`等3件が`bind: invalid
   argument`で失敗** — `t.TempDir()`が返すパスが、macOSでは`$TMPDIR`
   （`/var/folders/.../T/...`）由来で長く、AF_UNIXの`sun_path`上限
   （macOSは~104バイトとLinuxよりさらに短い）を超えていた。手元Linuxでは
   `/tmp`が短いため再現せず、`ubuntu-latest`でも再現しなかった。
   → `sandbox/listener_test.go`に`newTestRuntimeDir`ヘルパーを追加し、
   `/tmp`直下に短い名前で一時ディレクトリを作るようにした
   （Windowsはこの経路（XDG_RUNTIME_DIR）を仕様上使わないため、この
   ヘルパーを使うテストはWindowsでは`t.Skip`する）。

修正後、ローカルで`make check`・`make race`・`make test`すべて再度
green化を確認し、pushしてGitHub Actions 3OSマトリクス（`test`/`race`とも）が
全green になったことを確認した。

Step 5自体（E2Eスクリプト・CI設定）の実装内容は以下の通り。

- `testdata/modules/sender_once.wat` — 固定メッセージを宛先1へ1回送って
  終了するだけの送信専用モジュール（testing.mdの「一定回数送ったら終了する」
  の最小形）。E2Eのnode Aとして使う。
- `tests/stamp/main.go` — E2Eで使う「ベースバイナリにWASMをスタンプする」
  最小ツール。実ビルダー（フェーズ④）の代用であり配布物ではない。フッターの
  組み立てロジックは`sandbox`パッケージと共有しない
  （`.claude/rules/directory-structure.md`の「書き込みはビルダーの責務」
  という切り分けに合わせ、意図的に独立実装とした）。PLAN.md Step2の記録
  （「E2Eで恒常的なスタンプ手段が必要になった時点で改めて用意する」）の実行。
- `tests/e2e_basic.sh` — nodeA（送信専用）・nodeB（受信専用、`host_probe.wasm`）
  を実プロセスとして起動し、AF_UNIX経由でメッセージが届くことを確認する。
  **観測手段の工夫**：nodeBの"_start"は1通受け取るまでrecv(timeout_ms=-1)で
  ブロックし、受け取ると素直に終了する。どちらのプロセスもstdoutを持たない
  ため、「nodeBプロセスが自発的に終了すること」を届いたことの観測点とした。
  nodeBの起動直後はまだ待ち受けが始まっていない可能性があるため、
  nodeAの送信を届くまで（最大30回×0.2秒）リトライする設計にした
  （sendは無言で失敗するため、リトライ以外に確実な同期手段がない）。
  `.claude/rules/testing.md`の「複数プロセスを扱うE2Eの注意」に従い、
  `trap`でのPID後始末、`XDG_RUNTIME_DIR`を一時ディレクトリへ向ける、
  `set -euo pipefail`下でのコマンド置換・パイプの扱いに注意した。
  ローカルで複数回実行しフレーキーでないこと、プロセスが残らないことを
  確認済み。
- `Makefile`の`test`ターゲットを`tests/*.sh`を回す役割に一本化した
  （単体テストは`check`/`race`が担うため、`test`が単体テストを重複実行して
  いたStep1時点の状態を解消）。
- `.github/workflows/test.yml` — `test`ジョブ（ubuntu/macos/windows
  3OSマトリクス、`gofmt`確認→`go vet`→単体テスト→`tests/e2e_basic.sh`）と
  `race`ジョブ（ubuntu/macosのみ、`.claude/rules/testing.md`の方針通り
  windowsは対象外）を用意した。`make`はwindows-latestに標準で入っていない
  ため、ワークフロー内では`make`を経由せずgoコマンド・シェルスクリプトを
  直接呼ぶ形にした。

**未完了**：このワークフローはまだリモートへpushしておらず、GitHub Actions上で
実際にgreenになったことを確認できていない。**フェーズ①完了の判定の最後の項目
（3OSマトリクスがgreen）は、pushしてActionsの結果を見るまで達成とみなさない。**
push自体は`.claude/settings.json`で確認を要する操作のため、ユーザーの指示を
仰ぐこと。

Step 4（サンドボックス間通信）を完了した。

- `sandbox/address.go` — IDからソケットパスへの解決（仕様書§3.2）。
  `resolveSocketPath(goos, xdgRuntimeDir, localAppData, uid, id)`という
  内部関数にGOOSと環境変数を引数として切り出し、実行環境によらず
  Windows分岐まで含めて単体テストできるようにした。Windows分岐は
  `filepath.Join`（コンパイル先OSのセパレータを使ってしまう）を使わず
  手組みで`\`区切りにしている。
- `sandbox/framing.go` — 長さプレフィックス（u32 LE）でのフレーミング
  （仕様書§3.5）。`ReadFrame`は宣言長が`maxFrame`を超える場合、
  `io.CopyN(io.Discard, ...)`でペイロード分を読み捨ててストリームの同期を
  保ったまま`oversized=true`を返す（呼び出し側がログを出し次のフレームへ
  進める）。
- `sandbox/listener.go` — `Listen(id)`は仕様書§3.2の「既存ソケットファイルに
  接続を試み、応答があれば別プロセスが稼働中としてエラー、なければstaleと
  みなして削除してからbind」を実装。`Serve`はaccept loop
  で各接続をgoroutine化し、受信フレームをMailboxへ積む。オーバーサイズ
  フレームは受信側でもレート制限付きでログする
  （送信側とは独立した検証点。仕様書§3.5）。
- `sandbox/dest.go` — `DestTable`が宛先番号→IDの割り当てと、宛先ごとの
  遅延接続（初回送信時に確立、失敗時は次回再試行）を保持する。未割り当て・
  未接続の宛先への送信は仕様書§3.4通りログを出さず黙って捨てる。
- `sandbox/host.go`の`send`実装を、Step3の「常に無言破棄」スタブから
  `cfg.Dest.Send`呼び出しに置き換えた。
- `cmd/execsandbox/main.go` — 標準`flag`パッケージで`-n/--name`・
  `-d/--dest`（繰り返し可、`N=ID`）を実装。**仕様書§7.1の残りのオプション
  （-e/-v/-m/-b/-f/-l/-s/-t/-x/-q/-h/-V）と、エラーメッセージの
  `execsandbox:`接頭辞への統一はフェーズ②の作り込みで行う** —
  Step4時点でのフラグ解析エラーは`flag`パッケージ自身の出力（接頭辞なし）の
  まま許容している。`-n`指定時のみ`sandbox.Listen`+`sandbox.Serve`を
  goroutineで起動する。
- テスト: `sandbox/address_test.go`（3環境の path解決）、
  `sandbox/framing_test.go`（往復・オーバーサイズ時の同期維持・EOF）、
  `sandbox/listener_test.go`（実ソケットでのListen+DestTable往復、
  stale socket のクリーンアップ、二重listen の拒否）。`make race`も通過。
- **実際の動作確認**: ビルドした`cmd/execsandbox`を使い、(a)不正な`-d`値で
  即座にエラー終了、(b)同一`-n`での二重起動が
  `another instance is already listening`で拒否される、(c)`kill -9`後の
  stale socketが次回起動時に自動的に片付けられる、(d)**2つの実プロセス**
  （送信専用のnodeAと、`host_probe.wasm`を積んだ受信専用のnodeB）を実際に
  起動し、`-d 1=nodeB`で送ったメッセージをnodeBが`recv`で受け取り
  正常終了することを確認した。2プロセスを跨いだ本格的なE2Eスクリプト化・
  CI連携はStep 5の範囲のため、ここでは手動確認に留めている。

Step 3（最小の実行経路）を完了した。

- `sandbox/mailbox.go` — `Mailbox`（上限付きFIFOキュー、tail-drop）を実装。
  `Push`は上限到達時にtail-dropしレート制限付きでログ出力、`Recv(ctx, bufCap)`
  はメッセージがバッファに収まらない場合`data=nil`かつ必要サイズを返し
  **メッセージをキューに残す**（仕様書§5.3の「バッファ不足時はメールボックスに
  残る」を素直に満たすため、内部はチャネルではなくmutex保護のスライス＋
  通知チャネルで実装し、先頭を覗いてから収まる場合のみ取り除く設計とした）。
- `sandbox/ratelimit.go` — tail-drop・フレーム長超過向けの共通レート制限
  ロガー（初回即時、以降は一定間隔で累計数をまとめて出力。
  `.claude/rules/cli-output.md`）。
- `sandbox/host.go` — `RegisterHostModule`で`send`/`recv`/`max_frame`を
  wazeroの`execsandbox`ホストモジュールとして登録。
  - `send`: `MaxFrame`超過はログを残して破棄。宛先解決（`-d`）・AF_UNIX接続
    （Step 4）が未実装のため、それ以外はすべて「宛先未割り当て」として
    仕様書§3.4通り無言で破棄する。範囲外ポインタもクラッシュせず無視する。
  - `recv`: `timeout_ms`の負値（無限待ち）・0（即時）・正値（期限付き）を
    すべて`context.WithTimeout`で実装（PLAN保留事項が提案していた方式）。
    ただし外部からの強制キャンセル（`-t`等）との連携はフェーズ②で扱う。
    `meta_ptr`/`buf_ptr`の範囲外チェックをメールボックス操作の**前**に行い、
    ゲストの誤用でメッセージを失わないようにした。
  - `max_frame`: 設定値をそのまま返す。
- `cmd/execsandbox/main.go` — Step 2の「バイト数を報告するだけ」を、実際に
  `wazero`でゲストを実行する処理に置き換えた。ホスト関数登録後
  `rt.Instantiate`を呼ぶことで、wazeroの既定`StartFunctions`（`"_start"`）が
  ゲストのエントリポイントを自動実行する（WASI CLIモジュールの慣習。
  仕様書§5.6の表でも引数・環境変数等はWASI標準に委ねるとしており、
  ゲストはWASI Preview 1モジュールである前提のため、これに倣った）。
  `-b`/`-f`はまだ未実装のため、仕様書§7.1の既定値（1024通/1MiB）を
  ハードコードしている。**WASI（`wasi_snapshot_preview1`）自体はまだ
  組み込んでいない**（今回のテスト用モジュールが不要としていたため）。
  TinyGo製モジュールや`-e`/`-v`/`-s`等を扱うタイミングで追加する。
- `testdata/modules/host_probe.wat` — 本実装（Step1のspikeとは異なり仮実装
  ではない）のsend/recv/max_frameを検証する新モジュール。個別exportで
  パラメータを自由に変えたテスト（オーバーサイズ送信、バッファ不足recv等）と、
  `_start`経由の自動起動（本番の経路）の両方を1モジュールでカバーする。
- テスト: `sandbox/mailbox_test.go`（FIFO順序、tail-drop、バッファ不足で
  メッセージが残ること、タイムアウト、Push待ちの解除）、
  `sandbox/host_test.go`（max_frame、送信ログの有無、受信の各分岐、
  `_start`自動実行によるエコー）。`make race`も通過。
- **実際の動作確認**: `cmd/execsandbox`をビルドし、`host_probe.wasm`を
  スタンプして実行。ゲストの`_start`が`recv(timeout_ms=-1)`で正しく
  ブロックすることを確認した（このプロセス単体では他に`Push`する
  goroutineが存在しないため、Goランタイムの「all goroutines are asleep -
  deadlock」で停止する。これはバグではなく、AF_UNIXの受信ループ
  （Step 4で追加）がまだ存在しないために生じる、この段階で織り込み済みの
  状態である。Step 4でリスナーgoroutineが常駐するようになれば解消する）。

**フェーズ①/ Step 2 完了**

Step 2（スタンプ方式の実装）を完了した。

- `sandbox/footer.go` — フッター（Magic 8 + Version 4 + Offset 8 + Length 8 +
  Reserved 4、ビッグエンディアン）の読み出しを実装。`ExtractWASM(r io.ReaderAt,
  size int64) ([]byte, error)`。`ReadAt`のみを使いファイル全体は読み込まない。
  マジック不一致・サイズ不足は`ErrNotStamped`として区別し、
  offset/lengthがファイルサイズと矛盾する場合は別エラーとする破損検知も追加。
  Magicは`EXECSB01`（ExecDBの`EXECDB01`と区別）。
- `cmd/execsandbox/main.go` — ベースバイナリの骨格。`os.Executable()`から
  自己を開き、`ExtractWASM`で埋め込みWASMを取り出す。マジック不一致時は
  ビルダーの使用を促す英語エラーを出して`exit 1`（仕様書§6.2、
  `.claude/rules/cli-output.md`）。wazeroでの実行はまだ持たず、Step 3で
  現在の「見つかったバイト数を報告するだけ」の処理を置き換える。
- **フッターの書き込みは`sandbox/`に置かない**（`.claude/rules/
  directory-structure.md`の方針通り、書き込みはcmd/execsandbox-build＝
  フェーズ④の責務）。単体テスト用の組み立てヘルパーは`sandbox/footer_test.go`
  内の非公開関数として実装（本番ビルドには含まれない）。
- **実際の動作確認**：`cmd/execsandbox`をビルドし、(a)スタンプなしで実行して
  英語エラー＋`exit 1`を確認、(b)ビルダー代わりの一時的なGoスニペットで
  ダミーWASM＋フッターを追記したコピーを作り、実行して埋め込みバイト数が
  正しく報告されることを確認した（一時スニペットはリポジトリには残していない。
  Step 3以降、E2Eで恒常的なスタンプ手段が必要になった時点で改めて用意する）。
- `execdb_poc.go`を削除した。フッター読み出しの実装は完了し、ExecDBの書き込み
  側（自己上書き等）は`.claude/rules/stamp.md`の「引き継がない部分」に該当し
  元々不要だったため、参照価値がなくなったと判断した。

**フェーズ①/ Step 1 完了**

Step 1（足場固め＋技術検証）を完了した。

- `go.mod`（`github.com/amisonnet8/execsandbox`）、`wazero`依存の追加、
  `Makefile`（`build`/`test`/`check`/`fmt`/`fmt-check`/`race`/`testdata`）、
  `.gitignore`、`.gitattributes`、`LICENSE`（MIT）を配置。
- **スパイク検証【意思決定ゲート】: 合格。** `testdata/modules/spike.wat`
  （手書きWAT、`wat2wasm`でコンパイル）と `sandbox/wasm_spike_test.go`
  （`wazero`でホスト関数を直接登録）により、以下を実測で確認した。
  - ゲストが`execsandbox`モジュールの`send`/`recv`をimportできる。
  - `send(dest, ptr, len)`でホストがゲストの線形メモリを正しく読める。
  - `recv(meta_ptr, buf_ptr, buf_cap, timeout_ms)`でホストがゲストの線形
    メモリへ書き込める。特に`meta_ptr`が指す8バイト
    （`kind: u32 LE` + `conn_id: u32 LE`、仕様書§5.3）のレイアウトが
    想定通りに書き込み・読み出しできることを確認した。
  - ABI設計（仕様書5章）の見直しは不要と判断。

  このテストで確認したホスト関数実装パターンは、Step 3で`sandbox`パッケージの
  本実装（メールボックス連携）として作り込む。`sandbox/wasm_spike_test.go`は
  あくまで技術検証であり、本実装ではない。

- Step 1のなかで、以下の保留事項を決定した（詳細は「保留事項」参照）。
  - **Makefileのターゲット構成** → `build`/`test`/`check`/`fmt`/`fmt-check`/
    `race`/`testdata`の7つ。
  - **検証用WASMモジュールの管理方法** → 手書きWATを`wat2wasm`でコンパイルし、
    `.wat`ソースと`.wasm`成果物の両方をコミットする。CIに`wat2wasm`の導入を
    前提にしない。

**フェーズ①は完了した**（3OSマトリクスがWindowsでのAF_UNIX疎通を含めて
green）。次はフェーズ②（本体の作り込み：ポリシー適用・CLI・バックプレッ
シャー・ログ）に着手する。フェーズ②のステップ分割はまだ決めていないため、
着手前に提案する。

## 保留事項

以下は判断を先送りしている事項。該当するタイミングが来たら提案・相談すること。

- **`Makefile` のターゲット構成** — **Step 1で決定、Step 5で`test`の役割を
  確定。** `build`/`test`/`check`/`fmt`/`fmt-check`/`race`/`testdata`の7つ。
  `test`はStep1時点では単体テストの重複実行だったが、Step5で
  `tests/*.sh`（E2E）を走らせる役割に一本化した（単体テストは`check`/`race`が
  担う。`.claude/rules/directory-structure.md`の「`tests/`はmake testが実行する
  E2E/結合テスト一式」という定義に合わせた）。`build`は現時点では
  `go build ./...`のみ（`cmd/execsandbox`等が増えたら実体を伴う）。
  `.claude/settings.json`のビルド自動フックはまだ設定していない
  （実装がある程度進んでから提案する、`CLAUDE.md`参照）。
- **`execsandbox-sdk` リポジトリの立ち上げ時期** — フェーズ①完了（ABI確定）が
  前提だが、①の直後に始めるか、②③と並行させるかは未確定。並行させる場合、
  SDK側が本体の未実装機能（外部接続等）をラップできない期間が生じる。
  **立ち上げ時には `docs/spec/sdk_binding_ja.md` をそちらへ移設し、本リポジトリ
  からは削除すること**（`.claude/rules/directory-structure.md`）。SDKリポジトリの
  Description/Topicsは`.claude/rules/distribution.md`に暫定案を置いてあるので、
  実装する言語が確定した時点で見直すこと。
- **検証用WASMモジュールの管理方法** — **Step 1で決定済み。** 手書きWATを
  `wat2wasm`でコンパイルし、`.wat`ソースと`.wasm`成果物を両方コミットする
  （`.claude/rules/testing.md`「現時点のツールチェーン方針」）。`.wat`編集後は
  `make testdata`で再生成する。TinyGoは引き続き未導入（必要になった時点で追加）。
- **`recv` のタイムアウト実装方式** — **`timeout_ms`自体の意味論はStep 3で
  実装済み**（`context.WithTimeout`、負値=無限待ち・0=即時・正値=期限付き。
  `sandbox/host.go`）。残る論点は、`-t/--timeout`等でプロセス全体を強制終了
  する際に、`recv`でブロック中のゲストをどう安全に中断するか（wazero側の
  コンテキストキャンセルとの連携）。これはフェーズ②で`-t`を実装する際に
  確認する。
- **バージョン埋め込み** — ExecDBは `-ldflags -X main.version=` を使っていた。
  ExecSandboxでは本体とビルダーの2つにバージョンがあり、さらに生成物が
  「どのバージョンの本体でスタンプされたか」を持つ。フッターのversionフィールドと
  どう関係づけるかをフェーズ④で整理する。
