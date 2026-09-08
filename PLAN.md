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

## フェーズ③のステップ

外部接続であるフェーズ③は、以下の5ステップで進める。フェーズ②Step 1のような
意思決定ゲートは置かない——`WithCloseOnContextDone`のような未検証の外部依存はなく、
使うのは`net`パッケージと既存のメールボックスだけであるため。

**設計上の最大の要点**：現在の`Mailbox`は`[][]byte`しか持たず、`recv`のメタデータ
（`kind`/`conn_id`）は`sandbox/host.go`で0固定に書かれている。ここをイベント型
（`kind`/`conn_id`/ペイロード）に一般化することが土台であり、他のすべてがその上に
乗る（Step 1）。

**事前確認済みの設計判断**：
- **切断イベント（kind=3）はメールボックス上限を無視して必ず積む。** 仕様書§3.6は
  一律tail-dropと書く一方、§4.2は「切断イベントは必ず配送される」前提に立っており、
  素直に実装すると混雑時にゲストがconnIDごとの状態を永久に破棄できずリークする。
  上限超過分は「同時生存接続数」で上界が定まるためメモリ上界の保証は崩れない。
  この例外は仕様書§3.6への1文追記をStep 5で提案する。
- **`conn_write`の書き込み失敗は`-1`を返し、その接続を破棄する。** 接続を閉じて
  テーブルから削除し、切断イベントを配送してから`-1`を返す。以後そのconnIDは
  本当に「不明なconnID」になるため、§5.4の2値（`0`/`-1`）のままABIを変えずに済む。
- **`conn_write`の書き込み待ちには`-t`由来のdeadlineのみを反映する。** ホスト関数へ
  渡るctxがdeadlineを持つとき（`-t`指定時）だけ`SetWriteDeadline`し、未指定なら
  従来どおり完了までブロックする。

1. **Step 1: メールボックスのイベント型化（kind/conn_idの搬送）【土台】**
   - `sandbox/mailbox.go`: `Message{Kind, ConnID uint32; Payload []byte}`を新設し
     キューを`[]Message`へ。`PushDisconnect(connID)`は上限を無視して必ず積む。
     `Recv`の戻り値を`RecvOutcome`（`RecvDelivered`/`RecvTimedOut`/
     `RecvBufferTooSmall`）の3分岐に整理し、ABIの`recv`の戻り値と1対1対応させる。
   - `sandbox/host.go`の`recvFunc`をmsg.Kind/msg.ConnIDから書くよう変更。
   - `sandbox/listener.go`の`serveConn`を`Message{Kind: 0, ...}`へ追随。
   - 検証: 既存のE2E（`e2e_basic`/`e2e_policy`/`e2e_timeout`）を全て再実行し
     回帰がないことを確認。
2. **Step 2: `-l/--listen`のパースとアドレス解析（§7.4）**
   - `sandbox/listenaddr.go`（新設）: `ParseListenAddress(s string) (network,
     address string, err error)`。§7.4の全形式（ポート番号のみ／IPv4／`:PORT`／
     IPv6／Unixソケットパス／`unix:`接頭辞）を扱う純関数。Windowsドライブレター
     パスは`unix:`接頭辞を必須とする。
   - `options.go`に`-l/--listen`を追加。2回以上の指定はエラー（§4.1）。
   - 検証: §7.4の全形式＋異常系のテーブルテスト。
3. **Step 3: 接続管理とイベントのメールボックス合流（§4.2、§4.3）**
   - `sandbox/conn.go`（新設）: `ConnTable`。connIDは1始まり単調増加、切断後も
     再利用しない（u32一巡時は生存中のIDを飛ばす）。`Serve`が確立→kind=1、
     読み取り→kind=2、EOF/エラー→テーブルから削除しPushDisconnect。読み取り
     チャンク長は`min(maxFrame, 64KiB)`。読み取ったバイト列は必ずコピーしてから
     Push。
   - `main.go`: `-l`指定時に`net.Listen`し`go connTable.Serve(...)`。Unixソケットの
     stale掃除は行わない（`-n`とは対照的、`-l`は利用者が明示したパスのため）。
     接続の受理・切断はホスト側ログに出さない（ゲストへ既に通知済みの事象のため）。
   - 検証: `net.Listen("tcp", "127.0.0.1:0")`で確立→データ→切断の3イベントが
     正しいkind/conn_idで流れること、connIDが再利用されないこと、メールボックス
     満杯でも切断イベントが必ず積まれることを確認。
4. **Step 4: `conn_write`の実装（§5.4）**
   - `HostConfig.Conns *ConnTable`を追加し`conn_write`をエクスポート。`Conns`が
     nil（`-l`未指定）なら常に`-1`。
   - `ConnTable.Write`: 不明なconnID→`-1`、ctxがdeadlineを持てば
     `SetWriteDeadline`に反映、書き込み失敗は接続を閉じテーブルから削除し
     PushDisconnectしてから`-1`、成功は`0`。範囲外ポインタも`-1`。
   - `testdata/modules/conn_echo.wat`（新設）: kind=2ならconn_writeで書き戻し、
     kind=1/3および未知のkindは無視してループ継続。
   - 検証: 実際のTCPリスナー＋Goクライアントで`conn_echo.wasm`との往復、不明な
     connIDへの`-1`、切断済み接続への書き込みが`-1`になり切断イベントが流れる
     ことを確認。
5. **Step 5: E2E・ドキュメント・仕上げ**
   - `tests/connclient/main.go`（新設、Goで書く。3OSで同じ挙動を得るため`nc`に
     頼らない）、`tests/e2e_conn.sh`（新設、TCPを使いUnixソケットパスを引数に
     渡さずGit Bash/MSYSのパス変換を回避）。`.github/workflows/test.yml`に
     3OS分のステップを追加。
   - `docs/usage/execsandbox.md`を更新（`-l`実装済み化、§7.4書式表、kindの3種、
     切断イベントの上限無視、`conn_write`の挙動、staleソケット非掃除、
     Windowsの`unix:`接頭辞）。
   - 仕様書§3.6への1文追記を提案する（承認を得てから編集）。
   - `.claude/rules/naming.md`・`cli-output.md`の追記を検討。
   - 「現在地」をフェーズ③完了へ更新し、完了判定リストを記載する。

**検証方針（共通）**：各Stepとも`go build ./...`→`go vet ./...`→`gofmt -l .`→
単体テスト→実際にビルド・スタンプした実行ファイルでの目視確認、を最小単位とする。
`make check`/`make race`/`make test`をStep 1完了時（土台の変更が既存全体に及ぶ
ため）・Step 5完了時に通す。コミットは各Step完了時に行う。pushは行わない。

## フェーズ④のステップ

ビルダーとリリースであるフェーズ④は、以下の6ステップで進める。
`cmd/execsandbox-build`はまだ1行も存在しない。ビルダーは仕様書§6.3により
自分自身に6環境分のベースバイナリを`//go:embed`で内包する設計であり、
「ビルダーはembed、生成物はスタンプ」という二層構造
（`.claude/rules/stamp.md`）を最初に成立させないとビルダー自体が
`go build`できない（embed対象のファイルが存在しないため）。

**事前確認済みの設計判断**：
- `go install`は非対応とする。配布はGitHub Releasesの6本のみ
  （`.claude/rules/distribution.md`の保留事項を解決）。
- ライセンス表示義務を果たす主体は、ビルダーではなく**生成物
  （`cmd/execsandbox`）側**に持たせる。第三者へ配布されるのは生成物であり、
  配布者がビルダーを手元に持っているとは限らないため。ビルダー側には
  同機能を追加しない。
- 短縮フラグは`-L`（`--print-licenses`）とする。`.claude/rules/naming.md`の
  原則（短縮形は小文字、`-V`のみ例外）に、`-V`＝バージョン／`-L`＝ライセンス
  という対比で新たな例外を追加する。
- 上記は仕様書§7.1・§10.1への変更を伴うが、計画段階で既に提案・承認済みの
  ため実装時に再確認は取らない。

1. **Step 1: ビルダーCLI足場 + クロスビルド土台（go:embedの成立）**
   - `Makefile`に`cross-base`（6環境分の`cmd/execsandbox`を
     `cmd/execsandbox-build/basebinaries/`へクロスビルド）を新設し、
     `build`/`check`/`race`を依存させる。`.gitignore`に
     `/cmd/execsandbox-build/basebinaries/`を追記。
   - `cmd/execsandbox-build/{main,options,basebinaries}.go`（新設）：
     `-o/--output`（必須）、`--target GOOS/GOARCH`（6通りのみ許可）、
     `-h`/`-V`。
2. **Step 2: スタンプ処理（書き込み側）の実装**
   - `cmd/execsandbox-build/stamp.go`（新設）：フッター書き込み。
     `tests/stamp`とは独立実装のまま残す。出力に実行ビット`0o755`
     （Windowsはテストをガード）。
   - 実機で生成物が実際に起動しWASMを実行できることまで確認。
3. **Step 3: バージョン埋め込みと`-L/--print-licenses`（生成物側）**
   - `cmd/execsandbox-build`に`var version = "dev"`を追加。公式リリースは
     6本のベースバイナリと6本のビルダーを同じ`-X main.version=<tag>`で
     一括ビルドすることで「ビルダーのバージョン＝生成物のバージョン」を
     保証する。
   - `cmd/execsandbox/licenses/`（新設）：`execsandbox.LICENSE`（リポジトリ
     直下`LICENSE`のコピー）、`wazero.LICENSE`・`wazero.NOTICE`
     （go.modのwazero v1.12.0からコピー）。go:embedの`..`参照不可のため
     複製が必要。`licenses.go`でembedし`-L`で出力。
   - 仕様書§7.1に`-L`行を追加、§10.1を実装済みとして書き直す。
     `.claude/rules/naming.md`の例外を`-V`と`-L`の2つへ更新。
     `docs/usage/execsandbox.md`を実測値で更新。
   - `licenses_test.go`でリポジトリ直下`LICENSE`との一致を検証しドリフトを
     検知する。
4. **Step 4: Makefileの結線とビルダーのE2E確認**
   - `tests/e2e_builder.sh`（新設）：実際にビルダーで検証用wasmをスタンプし、
     生成物が動くことを確認（自環境向けは実行確認、クロス生成分は
     バイト比較で担保）。`.github/workflows/test.yml`に3OS分追加。
5. **Step 5: リリースパイプライン（`.github/workflows/release.yml`新設）**
   - `v*`タグpushトリガー、`check`ジョブでゲート、`build`ジョブ
     （`ubuntu-latest`1台、`CGO_ENABLED=0`）で6+6本をクロスビルドし
     `sha256sum`付きで`gh release create`によりアップロード
     （サードパーティActionsを増やさず`gh` CLIのみ使用）。
6. **Step 6: README・ドキュメント仕上げ**
   - `README.md`（新設）：クイックスタートのみ、詳細は`docs/`へリンク。
   - `docs/usage/execsandbox-build.md`を実測値へ差し替え。
   - リポジトリのメタデータ（Description/Topics）はユーザーが設定する
     （案内のみ行う）。
   - 「現在地」をフェーズ④完了へ更新。保留事項「バージョン埋め込み」等を
     解決済みへ更新する。

**検証方針（共通）**：各Stepとも`go build ./...`→`go vet ./...`→
`gofmt -l .`→単体テスト→実際にビルドしたビルダーでの目視確認、を最小単位
とする。`make check`/`make race`/`make test`をStep 1完了時（go:embedの
土台が既存全体に影響するため）・Step 6完了時に通す。コミットは各Step完了時
に行う。pushは行わない。タグのpush・実際のリリース作成は本フェーズの
範囲外とし、release.ymlはワークフロー定義のレビューまでに留める。

## 現在地

**`docs/tour/`（入門ガイド）を新設した（2026-09-08）。これで`docs/`の
4系統（`spec`/`usage`/`examples`/`tour`）がすべて揃った。**

先行プロジェクトExecDBのtourはこの環境になく参照できなかったため、
ユーザーが挙げた"A Tour of Go"（go.dev/tour）のイメージ——1ページ=1概念の
小さなステップを積み重ね、前の章を前提に次へ進む構成——を指針にした。
`docs/examples/`（下記）で既に実測済みのコマンド・出力を、5セクション16章
に分割・再構成して物語順に並べ直したもので、新規のビルド・実行検証は
中間ステップ（「既定では何もできない」「送信は届くことを保証しない」の
2箇所）のみで済んだ。

- `docs/tour/README.md`（目次、5セクション：はじめに／サンドボックス間
  通信／外部接続／リソースとポリシー／おわりに）と`01-what-is-execsandbox.md`
  〜`16-where-to-go-next.md`の16章。各章末に前後の章へのナビゲーション
  リンクを付けた。
- リンク切れがないことをスクリプトで確認済み。
- `README.md`の「ドキュメント」節に`docs/tour/`へのリンクを追加した
  （先頭、「初めての人向け」の案内として）。
- ドキュメントのみの追加のため`make check`への影響はない想定
  （確認は次のコミット前に実施）。

`docs/examples/`作成時に一時導入したTinyGo 0.42.0・Rust(stable)+
`wasm32-wasip1`はこのセッションでも継続して使えた（`/tmp/execsandbox-work/`
のビルダーも再利用）。

**フォローアップ**: ユーザーから、リリース作成・リポジトリメタデータ設定
（上記「保留事項」参照）は既に完了済みとの確認を得た。`execsandbox-sdk`の
パッケージマネージャ公開はSDK側のリポジトリ・devcontainerで行う（本
リポジトリのスコープ外）。`docs/tour/`執筆中に見つけた「TinyGoの
`crypto/rand`が`-x random`のエラーを握りつぶす」という知見は、新設した
`.claude/rules/guest-quirks.md`（ゲスト言語のWASI実装に起因する落とし穴を
集めるファイル。`wazero-quirks.md`のゲスト言語版）に切り出した。
`CLAUDE.md`の参照リストにも追加済み。

---

以下は`docs/examples/`新設時点の記録。

**`docs/examples/`（実例集）を新設した（2026-09-08）。** フェーズ①〜④
すべて完了済み、姉妹リポジトリ`execsandbox-sdk`（TinyGo版・Rust版）も実装・
CI green・E2E確認まで一段落したことを受け、`.claude/rules/
directory-structure.md`が定める作成タイミング（`docs/examples/`はフェーズ③
完了後、`docs/tour/`は全フェーズ完了後）を両方満たしたためユーザーに相談し、
`docs/examples/`→`docs/tour/`の順で進めることに合意した。今回は
`docs/examples/`のみ。

- `docs/examples/README.md`（索引）と5本の実例
  （`hello-wasi.md`・`sandbox-messaging.md`・`external-connection.md`・
  `policy-and-limits.md`・`polyglot-messaging.md`）を新設。すべて実際に
  ビルド・スタンプ・実行して出力を実測した上で記載した（このセッションに
  TinyGo 0.42.0とRust stable+`wasm32-wasip1`ターゲットを一時導入した——
  `execsandbox-sdk/.devcontainer/`と同じ手順。本体側の`devcontainer.json`
  自体は変更していないため、次回このdevcontainerを再構築すると消える。
  TinyGoが継続的に必要になった場合は`.claude/rules/testing.md`の
  「現時点のツールチェーン方針」の見直しを検討すること）。
- `hello-wasi`・`policy-and-limits`の4シナリオ（ファイルアクセス・メモリ
  上限・タイムアウト・乱数時刻の遮断）はSDKを使わない新規のTinyGoゲストを
  `docs/examples/src/`に書き下ろした。`sandbox-messaging`・
  `external-connection`・`polyglot-messaging`は`execsandbox-sdk`側の既存
  examples（`go/examples/{sender,receiver,echo}`・`rust/execsandbox/
  examples/{sender,receiver}.rs`）をそのまま使い、本リポジトリにコードを
  複製していない。
- **重要な発見**: `-x random`はホストのABI境界（`random_get`）では確実に
  遮断できるが、TinyGoの`crypto/rand`（wasip1ターゲット）は`random_get`を
  直接呼ばず、戻り値を持たないlibc関数`arc4random_buf`を経由するため、
  エラーがゲストに伝わらず「常に同じ固定値（実測では`117`）を返す」という
  形で観測される（`docs/examples/policy-and-limits.md`に実測込みで記載）。
  ExecSandbox本体のバグではなくゲスト言語のlibc実装に起因する挙動。
- 副産物として`docs/spec/sdk_binding_ja.md`（SDKバインディングの暫定設計
  文書）を削除した。当初計画は`execsandbox-sdk`への移設だったが、同リポジトリの
  各パッケージREADMEが実測ベースの後継として既に十分な内容を持ち、暫定文書の
  想定（`RecvTimeout`/`SendAll`等）は実装済みAPIと差分があったため、移設せず
  削除するとユーザーが判断した（詳細は上記「保留事項」、経緯は
  `.claude/rules/directory-structure.md`に記載）。
- `README.md`の「ドキュメント」節に`docs/examples/`へのリンクを追加した。
- **実際の動作確認**: 5本すべてを実機でビルド・スタンプ・実行し、`.md`内の
  コマンド出力はすべて実測値。`make check`（`go vet`・`go test`・
  `gofmt -l`）はexamples追加後もgreenのまま（`docs/examples/src/`配下は
  独立した`go.mod`を持つため本体のモジュールツリーに含まれない）。

次は`docs/tour/`（入門ガイド）——上記の通り、直後のセッションで着手・完了した。

---

以下はフェーズ④完了時点の記録。

**フェーズ④（ビルダーとリリース）は完了した。ExecSandboxの4フェーズすべてが
完了し、GitHub Releasesでリリースできる状態になった。**

Step 1〜6を完了した。

- **Step 1（ビルダーCLI足場 + クロスビルド土台）**: `cmd/execsandbox-build`
  （新設）に`-o/--output`（必須）・`--target GOOS/GOARCH`（仕様書§6.3の6環境
  のみ許可、省略時は`runtime.GOOS`/`GOARCH`から自動判定）・`-h`/`-V`を実装。
  ビルダーは仕様書§6.3により自分自身に6環境分のベースバイナリを
  `//go:embed`で内包するため（`basebinaries.go`）、`Makefile`に`cross-base`
  ターゲット（6環境分の`cmd/execsandbox`を`cmd/execsandbox-build/
  basebinaries/`へクロスビルド）を新設し、`build`/`check`/`race`を依存
  させた。埋め込み対象は配布物のためコミットせず`.gitignore`へ追加。
  CIワークフロー（`test.yml`）は`make`を経由しない方針のため、`test`/`race`
  両ジョブに同内容の「build base binaries」ステップを追加した。
- **Step 2（スタンプ処理・書き込み側の実装）**: `stamp.go`（新設）で
  フッター書き込み（読み出し側`sandbox.ExtractWASM`とは独立実装のまま、
  `.claude/rules/directory-structure.md`）。出力に実行ビット`0o755`。
  `--target windows`時は出力名に`.exe`を自動付与。実機で、ビルダーが
  実際にスタンプした実行ファイルが起動しAF_UNIX経由で送受信できることを
  確認した。
- **Step 3（バージョン埋め込みと`-L/--print-licenses`）**: 両バイナリに
  `var version = "dev"`（`-ldflags -X main.version=`で上書き）。**表示義務を
  果たす主体を、ビルダーではなく生成物（`cmd/execsandbox`）側に持たせる**
  というユーザー提案の設計を採用——第三者へ配布されるのは生成物であり、
  配布者がビルダーを手元に持っているとは限らないため。`cmd/execsandbox/
  licenses/`（新設）にリポジトリ直下`LICENSE`・wazero（v1.12.0）の
  LICENSE/NOTICEを複製し`go:embed`（`..`参照不可のため複製が必要）。
  短縮フラグは`-L`——`-V`と対になる**任意の**例外（`-p`等も選べたが、
  情報表示系オプションの一覧性のため大文字で揃えた。`-V`は`-v`が塞がって
  いるための**必然的な**例外であり性質が異なる）。仕様書§7.1・§10.1、
  `.claude/rules/naming.md`を更新（ユーザー承認済み、実装時再確認なし）。
  `licenses_test.go`でリポジトリ直下`LICENSE`・wazeroのモジュールキャッシュ
  とのドリフトを検知するテストを追加。
- **Step 4（Makefileの結線とビルダーのE2E確認）**: `tests/e2e_builder.sh`
  （新設）で、自環境向け（`--target`省略）にスタンプした実行ファイルが
  実際に送受信できること、6環境全ターゲットの生成物が「embedされた
  ベースバイナリ＋wasm＋フッター」の結合とバイト一致することを確認。
  `make test`を`cross-base`に依存させ、`test.yml`に3OS分のE2Eステップを
  追加した。
- **Step 5（リリースパイプライン）**: `.github/workflows/release.yml`
  （新設）。`v*`タグpushをトリガーに、`check`ジョブ（gofmt/vet/unit tests、
  `make check`相当）でゲートしてから`build`ジョブ（`ubuntu-latest`1台、
  `CGO_ENABLED=0`）で6環境分のベースバイナリ→6環境分のビルダーを同じ
  `-X main.version=${{ github.ref_name }}`でクロスビルドし、各アセットに
  `sha256sum`を添付、`gh` CLI（サードパーティActions不使用）で
  `execsandbox-build_<tag>_<goos>_<goarch>`の命名で公開する。ローカルで
  バージョン文字列付きビルド・チェックサム生成を再現し動作を確認したが、
  **実際のタグpush・リリース作成はこのセッションでは行っていない**
  （本フェーズの範囲外、確認済み方針）。
- **Step 6（README・ドキュメント仕上げ）**: `README.md`（新設）——
  インストール（GitHub Releasesから）とクイックスタートのみに絞り、詳細は
  `docs/`へリンク（`.claude/rules/directory-structure.md`の方針）。
  `docs/usage/execsandbox-build.md`を実測値へ全面差し替え（実際の`--help`
  出力、`go install`非対応の明記とその理由、`-L`による表示義務の果たし方、
  終了コード表）。リポジトリのメタデータ（Description/Topics）は
  `.claude/rules/distribution.md`に確定済みの文面があるが、**GitHub側の
  設定はユーザーが行う**（案内のみ行い、実際の変更はしていない）。

**実際の動作確認（フェーズ④全体）**: `go build`/`go vet`/`gofmt -l`/
`go test`/`make check`/`make race`/`make test`（`e2e_basic`/`e2e_builder`/
`e2e_conn`/`e2e_policy`/`e2e_timeout`の5本）すべてgreen。実機で、(1)自環境
向けにスタンプした実行ファイルの実際の送受信、(2)`--target windows/amd64`
での`.exe`自動付与、(3)6環境全ターゲットの生成物のバイト一致、(4)`-V`への
バージョン文字列の反映（両バイナリ）、(5)`-L`の出力内容（MIT全文・
Apache-2.0全文・NOTICE、計232行）を確認した。

**フェーズ④完了の判定**：
- ビルダー（`cmd/execsandbox-build`）が仕様書§6.1の全オプション
  （`-o`/`--target`/`-h`/`-V`）を実装し、6環境分のベースバイナリを実際に
  内包している。 ✅（Step1〜2）
- ビルダーが生成した実行ファイルが、実際に起動しWASMを実行できる。
  ✅（Step2、実機確認）
- 生成物自身が表示義務を果たす手段（`-L, --print-licenses`）を持つ。
  ✅（Step3）
- 6環境すべてのクロスターゲット生成が、embedされたベースバイナリと
  バイト一致する。 ✅（Step4）
- リリースパイプラインが定義され、タグ名がビルダー・生成物双方の
  バージョンとして一致する設計になっている。 ✅（Step5。実際のリリース
  作成は未実施）
- README・利用者向けドキュメントが実測値と一致している。 ✅（Step6）
- CI（3OSマトリクス、test/race）がすべてgreen。 ✅

**フェーズ④（ビルダーとリリース）は完了した。ExecSandboxの実装計画
（①〜④）はすべて完了した。** 残る作業は、ユーザーによる実際のタグpush・
リリース作成、リポジトリメタデータの設定、`execsandbox-sdk`リポジトリの
立ち上げ判断など、いずれも本リポジトリでのコード実装を伴わないもの
（下記「保留事項」参照）。

---

以下はフェーズ③の記録。

Step 5（E2E・ドキュメント追随・仕上げ）を完了した。

- `tests/connclient/main.go`（新設）: 外部接続E2E用の最小TCPクライアント。
  `nc`はwindows-latestランナーに存在しないため、3OSで同じ挙動を得るためGoで
  書いた（`tests/stamp`と同じ「配布物ではない補助ツール」という位置づけ）。
  接続してメッセージを送り、同じバイト列がエコーされることを確認する
  （終了コード：dial失敗=1〔リトライ可能〕、プロトコル不一致=2）。
- `tests/e2e_conn.sh`（新設）: `conn_echo.wasm`を`-l <port> -t 20s`で起動し、
  `connclient`から接続してエコーを確認する。TCPを使いUnixソケットパスを
  引数に渡さないことで、Git Bash/MSYS（windows-latest）のパス自動変換
  （`.claude/rules/testing.md`）を回避した。ポートは固定値（18902）を使用。
  `-t 20s`は、万一疎通に失敗してもプロセスが期限内に自己終了する安全網
  （trapによるkillとは独立、`.claude/rules/testing.md`「複数プロセスを
  扱うE2Eの注意」）。`.github/workflows/test.yml`に3OS分のステップとして
  追加した。
- `docs/usage/execsandbox.md`を更新: `-l`を実装済みとして全面的に書き直し
  （§7.4の書式表、Unixソケットパス・`unix:`接頭辞・Windowsドライブレターの
  扱い、staleソケットを自動的に片付けないこと、二重指定がエラーになること）。
  `recv`の`kind`（1=確立、2=データ、3=切断）の節、切断イベントがメールボックス
  上限を無視して必ず配送される旨、`conn_write`の戻り値と書き込み失敗時の
  挙動、`--timeout`指定時のみ書き込み待ちにも期限が適用される旨を追加した。
- **仕様書§3.6への追記を提案し、承認を得て反映した。** 「例外：外部接続の
  切断イベント（4.2、kind=3）は、この上限を無視して必ず配送する」という
  1文を追加した（Step 1で実装した挙動と仕様書の記述を一致させるため）。
- `.claude/rules/cli-output.md`に「動的でも、ゲストに既に通知済みの事象は
  出さない」という判断基準を追記した。既存の「静的か動的か」だけでは、
  外部接続の確立・切断（動的だがログを出さない）を説明できなかったため。
  `.claude/rules/naming.md`の対応表は設計時点から`conn_write`/`-l`の関係を
  正しく記載済みであり、追記は不要と判断した。
- **実際の動作確認**: `tests/e2e_conn.sh`を単体で複数回実行しフレーキーで
  ないこと・孤児プロセスが残らないことを確認。`make test`（`e2e_basic`/
  `e2e_conn`/`e2e_policy`/`e2e_timeout`の4本）、`make check`/`make race`
  すべてgreen。実機でstaleなUnixソケットファイルが起動時エラーになることを
  確認し、ドキュメントの記述と一致することを検証した。

**フェーズ③完了の判定**：
- `-l/--listen`が仕様書§7.4の全書式（ポート番号のみ・HOST:PORT・`:PORT`・
  IPv6・Unixソケットパス・`unix:`接頭辞）を受理し、2回以上の指定を拒否する。
  ✅（Step2）
- 外部接続の確立・データ・切断がメールボックスへイベント（`kind=1/2/3`）と
  して合流し、`recv`経由でゲストへ届く。✅（Step1・Step3）
- 切断イベントはメールボックス上限を無視して必ず配送される。✅（Step1・
  Step3、仕様書§3.6に反映済み）
- `conn_write`が実装され、書き込み成功・不明なconnID・書き込み失敗（接続の
  破棄と切断イベント配送を伴う）のいずれも仕様書§5.4の2値のまま扱える。
  ✅（Step4）
- `-t/--timeout`指定時、`conn_write`の書き込み待ちにも期限が適用される。
  ✅（Step4）
- E2E（`tests/e2e_conn.sh`）が3OSで外部接続の確立からエコーまでを確認する。
  ✅（Step5）
- CI（3OSマトリクス、test/race）がすべてgreen。✅
- `docs/usage/`が実測値と一致している。✅（Step5）
- `docs/spec/`との齟齬（切断イベントの上限例外）が解消されている。✅（Step5）

**フェーズ③（外部接続）は完了した。** 次はフェーズ④（ビルダーとリリース：
ベースバイナリ同梱、クロスターゲット、リリースパイプライン）に進む。
フェーズ④の計画立案は、着手時に改めて提案する。

---

以下はフェーズ②の記録。

Step 8（E2E拡張・ドキュメント追随・仕上げ）を完了した。

- `tests/e2e_policy.sh`（新設）: 単一プロセスで完結するCLI基本動作の
  E2E確認。`-s out`/`-e`/`--`以降の引数が`wasi_probe`へ実際に届くこと、
  `-s`未指定では標準出力が空のままであること、`--help`/`-V`がexit 0で
  stdoutに出ること、未定義オプションがexit 2になること、`-q`がフレーム長
  超過ログを抑制すること（`sender_once`を`-f 8`で送信させ、16バイト固定の
  メッセージを確実にoversizedにすることで決定的に再現。宛先が実在しなくても
  `send`側の`MaxFrame`検査が先に働くため、複数プロセスを起動せずに再現
  できる）。`.github/workflows/test.yml`に3OS分のステップとして追加した。
- `.claude/rules/testing.md`に、Git Bash/MSYS（`windows-latest`）が
  コマンドライン引数中の`/`始まりの文字列を自動的にWindowsパスへ変換して
  しまう罠を追記した。現時点では`-v`を使うE2Eスクリプトがまだ無く実害は
  出ていないが、今後`docs/examples/`や`tests/`に`-v`の実例を足す際に
  踏まえるべき事項として先回りで記録した（対策は該当コマンドにのみ
  `MSYS_NO_PATHCONV=1`を付与する）。
- `docs/usage/execsandbox.md`を実測値へ全面的に差し替えた。実際の
  `--help`出力の埋め込み、`-l/--listen`が未実装であることの明記
  （フェーズ③まで`--help`にも現れない）、`-m`/`-f`の実際の上限
  （4GiB／2GiB−1、いずれも実装で判明した制約）、`-v`のホストパス存在確認・
  Windowsドライブレター対応、`-d`の重複拒否、`-t`の専用節（終了コード124、
  および「`recv`の戻り値を確認するゲストほど自発的に終了できる」という
  Step 7の発見）、ホスト側ログの実際の文言、起動時オプションの誤りと
  終了コードの対応表（`--help`/`-V`→0、オプションエラー→2、起動失敗→1、
  タイムアウト→124、ゲスト自身の終了コード→そのまま、正常終了→0）を追加した。
  `docs/spec/`は変更していない。
- `.claude/rules/wazero-quirks.md`の新設を提案する（下記、まだ作成していない。
  CLAUDE.mdの「提案するだけで、勝手に作成・適用はしない」に従い、ユーザーの
  判断を待つ）。
- **実際の動作確認**: `tests/e2e_policy.sh`単体、`make test`（`e2e_basic`/
  `e2e_policy`/`e2e_timeout`の3本）、`go build`/`go vet`/`gofmt -l`/
  `go test`/`make race`/`make check`すべてgreen。`docs/usage/`に追加した
  コマンド例・エラー出力例は、実際にビルドした実行ファイルで再現し
  文言を一致させた（`-m bogus`のエラーメッセージなど）。

**フェーズ②完了の判定**：
- 仕様書§7.1の全CLIオプションが実装され、パース・検証済みである
  （`-l/--listen`のみフェーズ③スコープとして意図的に除外）。 ✅
- 起動時オプションで指定しない限り、WASMモジュールは何もできない
  （環境変数・ファイルシステム・stdio・乱数・時刻いずれも既定遮断/既定の
  安全側）。 ✅（Step 4〜6。乱数・時刻は「許可が既定」という仕様上の例外を
  wazeroの逆転した既定に逆らって正しく成立させた）
- ホスト側ログが`execsandbox:`接頭辞に統一され、`-q`で抑制できる
  （起動エラーを除く）。 ✅（Step 3）
- `-b`/`-f`によるバックプレッシャーが実際のオプション値で動く。
  ✅（Step 2/3、実機でtail-dropを確認）
- `-m`によるメモリ上限、`-v`によるファイルシステムアクセス制御が機能する。
  ✅（Step 5、実機でホストファイルへの実際の書き込み・読み取り専用の拒否を
  確認）
- `-t`による実行時間の強制終了が機能し、終了コードが体系立っている。
  ✅（Step 7、`tests/e2e_timeout.sh`で3OS確認）
- CI（3OSマトリクス、test/race）がすべてgreen。 ✅
- `docs/usage/`が実測値と一致している。 ✅（Step 8）

**フェーズ②（本体の作り込み）は完了した。** 次はフェーズ③（外部接続、
`-l/--listen`・`conn_write`）に進む。フェーズ③の計画立案は、着手時に
改めて提案する。

Step 7（`-t/--timeout`の本実装と終了コード整理）を完了した。

- `sandbox/policy.go`: `Policy.Timeout time.Duration`を追加。`RuntimeConfig()`
  は`Timeout > 0`のときだけ`WithCloseOnContextDone(true)`を呼ぶ
  （タイムアウトを使わない起動では常時有効化しておく理由がないため）。
- `main.go`: `run(opts *options) (exitCode int, err error)`へシグネチャ変更
  （従来は`error`のみ）。ゲスト実行（`InstantiateWithConfig`）にのみ、
  `-t`指定時は`context.WithTimeout(ctx, opts.timeout)`由来のcontextを渡す
  （ホスト関数登録・リスナー起動は`context.Background()`側で行い影響しない）。
  戻りエラーを`errors.As`で`*sys.ExitError`と判定し、
  `ExitCodeDeadlineExceeded`ならLogger経由で`"execution timed out after
  <duration>"`をログしexitCode=**124**（Unixの`timeout(1)`コマンドに倣った
  慣習。終了コード体系は仕様書を変更せず実装コメントに留める、確認済み
  方針）で終了。それ以外の`*sys.ExitError`（ゲスト自身の`proc_exit`等）は
  `execsandbox:`接頭辞を付けずそのままプロセスの終了コードとして伝える
  （ホスト側のエラーではないため）。それ以外のエラー（トラップ等）は
  従来通り`execsandbox:`接頭辞＋exit 1。
  - **タイムアウトのログはLogger経由とし`-q`で抑制されるようにした**
    （tail-drop・フレーム長超過と同じ「動的に発生しうるイベント」として
    扱う。起動を中止する類のエラーではないため）。実機で`-t -q`により
    ログが消え、終了コード124は変わらないことを確認した。
- **実装中に判明した重要な挙動**: `-t`はコンテキストキャンセルを
  `InstantiateWithConfig`に渡すだけなので、**ゲストの`recv`呼び出しが
  ちょうどブロック中だった場合、`recv`自身がそのcontextの`Done()`を見て
  自発的に`-1`（タイムアウト相当）を返し、ゲストがそれを見て正常終了
  すれば、`WithCloseOnContextDone`による強制終了（trap）は一度も発動せず
  exitCode 0の正常終了になる。** 例えば`host_probe.wasm`（1回`recv`して
  結果を見て`send`するかどうか決める）は、メッセージが来ないまま`-t`の
  期限を迎えても穏やかに終了する。一方`blocker.wasm`（`recv`の戻り値を
  見ずに無限ループで呼び直す）は、`recv`が`-1`を返してもループが止まらない
  ため、`WithCloseOnContextDone`による強制終了（`ExitCodeDeadlineExceeded`、
  exitCode 124）に頼ることになる。**これはSDK設計・ゲスト実装上望ましい
  性質**（律儀に`recv`の戻り値をチェックするゲストほど、強制終了ではなく
  自発的な終了の機会を得られる）であり、バグではないが、E2Eの検証対象
  選びに直結する重要な発見だった（下記参照）。
- `testdata/modules/`に新規モジュールは追加していない（Step1の
  `blocker.wasm`/`spin_forever.wasm`を再利用）。
- `sandbox/policy_test.go`に`TestPolicy_runtimeConfig_timeoutInterruptsBlockingGuest`
  を追加。Step1のスパイク検証（生のwazero API）と異なり、`main.go`が実際に
  呼ぶ`Policy.RuntimeConfig()`経由の配線を検証する。
- `tests/e2e_timeout.sh`（新設）: **`blocker.wasm`を使う**（`host_probe.wasm`
  では上記の理由で強制終了経路を通らないため）。`-n`も付けて起動し、
  「1秒でexitCode 124になること」「経過時間が5秒未満であること」
  「ソケットファイルが残らないこと」を確認する。`.github/workflows/test.yml`
  に3OS分のステップとして追加した。
- **実際の動作確認**: 上記`tests/e2e_timeout.sh`に加え、実機で
  `proc_exit(42)`を呼ぶだけの最小WATモジュール（コミットしない使い捨て）を
  スタンプして実行し、プロセスの終了コードが42になる（ゲストの終了コードが
  ホストのエラー扱いされずそのまま伝わる）ことを確認した。`-t`を付けても
  期限内に自発的に終わるゲストはexitCode 0のままであることも確認した。
  `go build`/`go vet`/`gofmt -l`/`go test`/`make race`/`make check`/
  `make test`すべてgreen。
- 保留事項「`recv`のタイムアウト実装方式」を解決済みに更新した。

Step 6（乱数・時刻〔`-x/--deny`〕。wazeroの既定が仕様と逆転する箇所）を
完了した。

- `sandbox/policy.go`: `Deny{Random, Time bool}`を`Policy.Deny`に追加。
  `ModuleConfig()`で、`Deny.Random`がfalse（既定）なら`WithRandSource
  (crypto/rand.Reader)`、trueなら常にエラーを返す`alwaysErrorReader`
  （wazeroの決定的乱数をそのまま「遮断」として流用しない。予測可能な乱数を
  許してしまうため）を設定。`Deny.Time`がfalse（既定）なら
  `WithSysWalltime()`＋`WithSysNanotime()`＋`WithSysNanosleep()`を三点
  セットで有効化（nanotimeだけ有効化するとGoランタイムのsleep実装が
  ビジーループになる）。`Deny.Time`がtrueの場合は何もしない
  （WASIの`clock_time_get`にエラー経路がなく「取得拒否」を表現できない
  ため、wazeroの既定＝偽の単調時計のままにするのが実効的な遮断になる）。
- **wazeroの既定挙動を実測で確認**: `WithRandSource`未設定時は決定的
  （毎回同じバイト列）、`WithSysWalltime`未設定時は`FakeEpochNanos`
  （`internal/platform/time.go`で定義、**2022-01-01T00:00:00Z**）を起点に
  1回の呼び出しごとに1ms進むだけの偽時計になることをソースで確認した。
  当初「小さい値（0付近）になるはず」と想定していたが誤りで、実際は
  2022年基準の一見もっともらしい大きな値になる（実機確認時に判明。
  テストの正しさには影響しない――`time.Since`による「現在時刻との差」で
  判定しているため）。
- `main.go`: `opts.deny`（`denySet{Random,Time bool}`、Step2でパース済み）を
  `sandbox.Deny`へ変換して`Policy.Deny`へ配線。
- `testdata/modules/wasi_probe.wat`を拡張し、`random_get`/`clock_time_get`を
  直接import。`random_probe(buf,len)`・`clock_probe(id,result_ptr)`
  エクスポート（個別検証用）に加え、`_start`の末尾でも同じ2つを呼び
  stdoutへ生バイトのまま書き出す（実機確認用、`-s out`必須）。
- `sandbox/policy_test.go`に**回帰防止の要**を追加: 既定で`random_get`が
  実行のたびに異なるバイト列を返すこと、既定で`clock_time_get(realtime)`が
  `time.Now()`の5秒以内に一致すること、`-x random`で`random_get`が
  エラーになること、`-x time`で返る時刻が`time.Now()`から1時間以上
  乖離していること（＝実時刻ではなくwazeroの偽時計）を固定した。
- **実際の動作確認**: 実機で`-s out`付きスタンプ実行の生バイト出力を
  `python3`でデコードし、既定では乱数が実行のたびに変わり時刻が
  実時刻（2026年）に一致すること、`-x random`では乱数が常に全0バイトに
  なること、`-x time`では時刻が2022-01-01T00:00:00Zになることを確認した。
  `docs/usage/execsandbox.md`の`-x, --deny`節に、`-x time`が
  「取得を遮断」ではなく「wazeroの偽時計を見せ続ける」という実効的な
  意味になる旨を追記した。`go build`/`go vet`/`gofmt -l`/`go test`/
  `make race`/`make check`/`make test`すべてgreen。
- `.claude/rules/wazero-quirks.md`の新設は計画通りStep 8で提案する
  （`sandbox/policy.go`のコメントに「Step 8で新設予定」と記載済み）。

Step 5（ファイルシステム〔`-v`〕とメモリ上限〔`-m`〕）を完了した。

- `sandbox/policy.go`: `Mount{Host, Guest, ReadOnly}`を`Policy.Mounts`に追加。
  `ModuleConfig()`は`Mounts`が空なら`WithFSConfig`を一切呼ばない（wazeroの
  既定＝`path_open`等が`ENOSYS`になる、という遮断状態を維持）。空でなければ
  `WithDirMount`/`WithReadOnlyDirMount`を`ReadOnly`に応じて呼び分ける。
  `MemoryLimitPages(bytes) (uint32, error)`（バイト→64KiBページへ変換、
  端数は切り上げ、65536ページ〔4GiB、wazeroの既定上限〕超はエラー）と
  `Policy.RuntimeConfig() (wazero.RuntimeConfig, error)`（内部で
  `MemoryLimitPages`を呼び`WithMemoryLimitPages`を設定）を新設。
  `WithMemoryCapacityFromMax`は呼ばない（上限は天井であって先行確保する量
  ではないため、確認済み方針）。
- `main.go`: `wazero.NewRuntime(ctx)`を`policy.RuntimeConfig()`の結果を使う
  `wazero.NewRuntimeWithConfig(ctx, rtConfig)`へ変更したため、`policy`の
  組み立てをRuntime生成より前に移動。`opts.volumes`→`sandbox.Mount`の変換
  （`toSandboxMounts`）、`opts.memLimit`→`Policy.MemoryLimitBytes`を追加。
- `options.go`の`validate()`に、`-v`で指定したホストパスの存在確認
  （`os.Stat`、`-v`のパース自体はGOOS非依存の純関数のままにするため
  ここでは行わない）を追加。
- `testdata/modules/mem_hog.wat`（新設）: `memory.grow`失敗まで1ページずつ
  伸ばし続け、到達ページ数を返す。モジュール側に最大値を宣言しないため、
  純粋に`-m`（`WithMemoryLimitPages`）の効果だけを観測できる。
- `testdata/modules/wasi_probe.wat`を拡張し`path_open`/`fd_close`を追加。
  `write_probe(path,data)`エクスポート（プリオープンfd=3〔wazeroの規約で
  最初のマウントは3番、0〜2はstdio〕へ書き込みを試みる）を新設。`_start`が
  末尾でこれを"probe.txt"に対して呼び、結果を無視する（マウントがなければ
  黙って失敗するだけで良い、実機確認用）。
- **実際の動作確認**: `sandbox/policy_test.go`にマウントなし/rw/roの3パターン
  （`path_open`のerrno・ホスト側ファイルの有無を確認）、`MemoryLimitPages`の
  境界値テーブルテスト、`RuntimeConfig`経由で`mem_hog`の`grow_until_fail`が
  実際に指定ページ数で頭打ちになることを確認。実機では、一時ディレクトリを
  `-v host:/data`でマウントして`probe.txt`が実際にホストへ書き込まれる
  こと、`:ro`付きでは書き込まれないこと、`-v`なしでは何も起きないことを
  確認した。`-m`の異常値（4GiB超）が起動前にエラーになることも確認した。
  `go build`/`go vet`/`gofmt -l`/`go test`/`make race`/`make check`/
  `make test`すべてgreen。

Step 4（WASI組み込みとModuleConfig土台：`-s`/`-e`/`--`引数）を完了した。

- `sandbox/policy.go`（新設）: `Policy`構造体（`Env []EnvVar`・`Stdio`・
  `Args`・`Stdin`/`Stdout`/`Stderr`）と`ModuleConfig()`。書式解釈（cmd側）と
  wazeroへの翻訳（sandbox側）を分担する計画通りの切り分け。`Stdio`の各項目が
  falseのままなら対応する`With*`を一切呼ばず、wazeroの既定
  （Stdinはio.EOF、Stdout/Stderrはio.Discard）に委ねることで「明示的に
  有効化しない限り何もできない」原則を保った。`RuntimeConfig()`は当面
  追加していない（メモリ上限・乱数時刻・タイムアウトが実際にRuntimeConfig
  側を必要とするのはStep 5/6/7からのため、今この時点で中身のない
  メソッドを足すのは避けた。計画時点ではwazeroの実際のAPI配置
  ―`WithRandSource`等は実は`ModuleConfig`側にあり`RuntimeConfig`では
  ない―を確認できていなかった。使う段になったら該当Stepで追加する）。
- `main.go`: `wasi_snapshot_preview1.Instantiate(ctx, rt)`を追加。
  `rt.Instantiate`を`rt.InstantiateWithConfig(ctx, wasmBytes,
  policy.ModuleConfig())`へ変更。`opts.env`/`opts.stdio`/`opts.guestArgs`から
  `sandbox.Policy`を組み立てる（`stdioSet`→`sandbox.Stdio`は同一形状の構造体
  変換、`envVar`→`sandbox.EnvVar`は`toSandboxEnv`で変換）。
- `testdata/modules/wasi_probe.wat`（新設）: `args_get`/`environ_get`/
  `fd_write`を直接import。`_start`はargv_buf・改行・environ_buf・改行を
  1回の`fd_write`（iovec4本）でfd=1へ書き出す。個別検証用に`argc_probe`/
  `environc_probe`（件数のみを返す）も持つ。
- `sandbox/policy_test.go`（新設）: `-s out`なしではstdoutが空のまま
  （既定遮断）・`-s out`ありで`argv0="execsandbox"`＋`--`以降の引数＋
  環境変数が実際にゲストへ届くこと・stdin/stderr未接続でもエラーに
  ならないこと・`argc_probe`/`environc_probe`の件数一致を確認。
- **実際の動作確認**: `wasi_probe`をスタンプし、`-s out -e A=1 -- x y`の
  実行結果を`od -c`で確認（`execsandbox\0x\0y\0\nA=1\0\n`が得られた）。
  `-s`を付けない既定では標準出力が完全に空になることも確認した。
  `tests/e2e_basic.sh`を含む既存の単体テスト・E2Eを全て再実行し、WASI登録が
  既存モジュール（`host_probe`等）に影響しないことを確認した。`go build`/
  `go vet`/`gofmt -l`/`go test`/`make race`/`make check`/`make test`
  すべてgreen。

Step 3（ログの統一とバックプレッシャー〔`-q`/`-b`/`-f`〕の実配線）を完了した。

- `sandbox/log.go`（新設）: `Logger`型（`execsandbox:`接頭辞・改行付与・
  `-q`時の抑制を集約）。並行呼び出し（複数送信元からの`Push`、複数接続の
  `serveConn`）による出力の混線を避けるため内部に`sync.Mutex`を持つ。
- `mailbox.go`/`host.go`/`listener.go`の`io.Writer`引数を`*Logger`へ置き換え、
  `fmt.Fprintf(w, "execsandbox: ...\n", ...)`の直書きを`log.Printf(...)`へ
  統一（両ファイルの`fmt`/`io`importが不要になり削除）。
- `main.go`: `sandbox.NewLogger(os.Stderr, opts.quiet)`を`run()`内で1つ作り、
  `NewMailbox`・`Serve`・`HostConfig.Log`すべてに同じインスタンスを渡す。
  起動時エラー（パースエラー・`run()`のエラー）はこれまで通り`Logger`を
  経由せず直接`os.Stderr`へ書く（`-q`は起動エラーを抑制しない、確認済み
  方針の通り）。
- `options.go`の`validate()`に`-f/--max-frame`の上限チェック
  （`math.MaxInt32`超はエラー）を追加。ABIの`max_frame()`がi32を返すこと、
  `recv`の不足バッファ表現`-(len)-1`もi32に収める必要があること
  （`.claude/rules/abi-compatibility.md`）が理由。
- `-b`/`-f`自体のオプション値→`NewMailbox`/`HostConfig.MaxFrame`への配線は
  Step 2で前倒し済みだったため、本Stepでは行っていない（Step 2の記録参照）。
- **実際の動作確認**: `host_probe`（1通受けたら即終了するモジュール）を
  `-b 1`でスタンプ起動し、起動直後に20並列で送信することで実際に
  tail-dropを発生させ、`execsandbox: mailbox full, dropped 1 message(s)`が
  stderrに出ることを確認した。同条件に`-q`を付けるとstderrが完全に空に
  なることも確認した。`tests/e2e_basic.sh`（既存）で回帰がないことも確認。
  `sandbox/log_test.go`（新設）で`Logger.Printf`の接頭辞・改行・quiet抑制を
  単体テスト化。`go build`/`go vet`/`gofmt -l`/`go test`/`make race`/
  `make check`/`make test`すべてgreen。

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
- **`execsandbox-sdk` リポジトリの立ち上げ** — **解決済み（2026-09-08）。**
  別リポジトリ `execsandbox-sdk` としてTinyGo版・Rust版とも実装・単体テスト・
  examples・本体を使ったE2E・CIまで完了した（詳細は同リポジトリの`PLAN.md`
  参照）。これに伴い `docs/spec/sdk_binding_ja.md`（暫定設計文書）は
  **移設せず削除した**。実装後のSDK APIは同文書の想定と差分があり
  （`Recv`/`recv`はブロッキング版・タイムアウト版を1関数に統合、`SendAll`は
  未実装、`Kind`定数名を変更 等）、`execsandbox-sdk`側の各パッケージREADME
  （`go/execsandbox/README.md`・`rust/execsandbox/README.md`）が実測ベースの
  後継として十分な内容を備えていたため（`.claude/rules/directory-structure.md`
  に判断の経緯を記載）。SDKリポジトリのDescription/Topicsは
  `.claude/rules/distribution.md`の暫定案から見直しが必要かどうか、別途判断
  すること。
- **検証用WASMモジュールの管理方法** — **Step 1で決定済み。** 手書きWATを
  `wat2wasm`でコンパイルし、`.wat`ソースと`.wasm`成果物を両方コミットする
  （`.claude/rules/testing.md`「現時点のツールチェーン方針」）。`.wat`編集後は
  `make testdata`で再生成する。TinyGoは引き続き未導入（必要になった時点で追加）。
- **`recv` のタイムアウト実装方式** — **解決済み（フェーズ②Step 1で機構を
  確定、Step 7で実装完了）。** `timeout_ms`自体の意味論はStep 3で実装済み
  （`context.WithTimeout`、負値=無限待ち・0=即時・正値=期限付き。
  `sandbox/host.go`）。`-t/--timeout`によるプロセス全体の強制終了は、
  `wazero.RuntimeConfig.WithCloseOnContextDone(true)`（`-t`指定時のみ有効化。
  `sandbox/policy.go`の`RuntimeConfig()`）とゲスト実行への
  `context.WithTimeout`の組み合わせで実現した（詳細はPLAN.md「現在地」の
  Step 7の記録を参照）。
- **バージョン埋め込み** — **解決済み（フェーズ④Step3〜5）。**
  `cmd/execsandbox`・`cmd/execsandbox-build`とも`var version = "dev"`を持ち
  `-ldflags -X main.version=<tag>`で上書きする（`-V`で表示）。フッターの
  Versionフィールド（`sandbox/footer.go`のfooterVersion）とは無関係——
  あちらは「フッター形式自体のバージョン」（現在値1、レイアウトが変わった
  ときだけ上げる）であり、リリースのタグとは別概念（`.claude/rules/
  stamp.md`）。公式リリース（`.github/workflows/release.yml`）では、6環境分の
  ベースバイナリと6環境分のビルダーを同じ`-X main.version=${{
  github.ref_name }}`で一括ビルドすることで「ビルダーのバージョン＝
  生成物のバージョン」（仕様書§6.3）を保証している——ビルダー実行時に
  埋め込み済みbase binaryのバージョンを動的に書き換えることはできないため。
