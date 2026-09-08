;; フェーズ②Step4/5の検証用モジュール。
;;
;; SDKを介さず"wasi_snapshot_preview1"のargs_get/environ_get/fd_write/
;; path_open/fd_closeを直接importし、-e(環境変数)・"--"以降の引数・
;; -s out(stdout配線)・-v(ファイルシステムマウント)がゲストへ実際に
;; 届くことを確認する（.claude/rules/testing.md「生のABIを直接叩くこと」）。
;;
;; メモリレイアウト:
;;   0..4    : args_sizes_get の argc
;;   4..8    : args_sizes_get の argv_len
;;   8..12   : environ_sizes_get の environc
;;   12..16  : environ_sizes_get の environ_len
;;   16..272 : argv の各要素へのオフセット配列（最大64個分）
;;   272..4368   : argv_buf（null終端文字列を連結したもの）
;;   4368..4624  : environ の各要素へのオフセット配列
;;   4624..8720  : environ_buf（null終端"KEY=VALUE"を連結したもの）
;;   8720..8721  : 区切り用の改行1バイト（データセグメント）
;;   8730..8762  : fd_writeへ渡すiovec配列（8バイト×4）
;;   8762..8766  : fd_writeの書き込みバイト数（結果、未使用だが仕様上必須）
;;   9000..9004  : write_probeが開いたfdを受け取る領域(path_openの結果)
;;   9010..9018  : write_probe用iovec（1本）
;;   9020..9024  : write_probeのfd_write書き込みバイト数（結果、未使用）
;;   9100..9109  : データセグメント"probe.txt"（_startが書き込むパス）
;;   9120..9142  : データセグメント"written-by-wasi_probe"（_startが書き込む内容）
;;
;; _start: argv_bufと改行とenviron_bufと改行を、1回のfd_write（iovec 4本）で
;; fd=1（stdout）へまとめて書き出す。-s outが無指定の場合、wazeroの既定
;; Stdout（io.Discard）へ書かれるだけでエラーにはならない（何もしなければ
;; 何もできない、という既定遮断の挙動そのものを確認する）。続けて、
;; -vでマウントされていれば"probe.txt"への書き込みも試みる（実機確認用。
;; マウントなしでは黙って失敗するだけで_start自体はエラーにしない）。
;;
;; write_probe: -vでマウントした最初のディレクトリ（プリオープンfd=3。
;; wazeroはfd 0/1/2をstdio、3以降をpreopenへ割り当てる）へ、指定した
;; パスへ指定バイト列を書き込む。マウントなし・読み取り専用の場合は
;; path_open/fd_writeがエラー(0以外)を返すことでも確認できる。
(module
  (import "wasi_snapshot_preview1" "args_sizes_get" (func $args_sizes_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "args_get" (func $args_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "environ_sizes_get" (func $environ_sizes_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "environ_get" (func $environ_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "fd_write" (func $fd_write (param i32 i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "path_open" (func $path_open (param i32 i32 i32 i32 i32 i64 i64 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "fd_close" (func $fd_close (param i32) (result i32)))

  (memory (export "memory") 1)
  (data (i32.const 8720) "\0a") ;; 改行1バイト
  (data (i32.const 9100) "probe.txt")
  (data (i32.const 9120) "written-by-wasi_probe")

  ;; write_probe(path_ptr, path_len, data_ptr, data_len) -> i32
  ;;   0: 成功
  ;;   正の値: path_openのerrno
  ;;   負の値: fd_writeのerrno(絶対値)。path_open成功後にfd_writeが
  ;;           失敗するケースは今のところ想定していないが、区別できるようにする。
  (func $write_probe (export "write_probe")
        (param $path_ptr i32) (param $path_len i32) (param $data_ptr i32) (param $data_len i32)
        (result i32)
    (local $errno i32)
    (local $fd i32)
    ;; oflags=O_CREAT(1)|O_TRUNC(8)=9, rights=RIGHT_FD_WRITE(64)
    (local.set $errno
      (call $path_open
        (i32.const 3) (i32.const 0)
        (local.get $path_ptr) (local.get $path_len)
        (i32.const 9) (i64.const 64) (i64.const 0)
        (i32.const 0) (i32.const 9000)))
    (if (i32.ne (local.get $errno) (i32.const 0))
      (then (return (local.get $errno))))

    (local.set $fd (i32.load (i32.const 9000)))
    (i32.store (i32.const 9010) (local.get $data_ptr))
    (i32.store (i32.const 9014) (local.get $data_len))
    (local.set $errno (call $fd_write (local.get $fd) (i32.const 9010) (i32.const 1) (i32.const 9020)))
    (drop (call $fd_close (local.get $fd)))
    (if (i32.ne (local.get $errno) (i32.const 0))
      (then (return (i32.sub (i32.const 0) (local.get $errno)))))
    (i32.const 0))

  ;; argc_probe/environc_probe: 個別検証用。件数のみを返す。
  (func (export "argc_probe") (result i32)
    (drop (call $args_sizes_get (i32.const 0) (i32.const 4)))
    (i32.load (i32.const 0)))

  (func (export "environc_probe") (result i32)
    (drop (call $environ_sizes_get (i32.const 8) (i32.const 12)))
    (i32.load (i32.const 8)))

  (func (export "_start")
    ;; argv_buf・environ_bufを埋める。
    (drop (call $args_sizes_get (i32.const 0) (i32.const 4)))
    (drop (call $args_get (i32.const 16) (i32.const 272)))
    (drop (call $environ_sizes_get (i32.const 8) (i32.const 12)))
    (drop (call $environ_get (i32.const 4368) (i32.const 4624)))

    ;; iovec[0] = {ptr: argv_buf, len: argv_len}
    (i32.store (i32.const 8730) (i32.const 272))
    (i32.store (i32.const 8734) (i32.load (i32.const 4)))
    ;; iovec[1] = {ptr: 改行, len: 1}
    (i32.store (i32.const 8738) (i32.const 8720))
    (i32.store (i32.const 8742) (i32.const 1))
    ;; iovec[2] = {ptr: environ_buf, len: environ_len}
    (i32.store (i32.const 8746) (i32.const 4624))
    (i32.store (i32.const 8750) (i32.load (i32.const 12)))
    ;; iovec[3] = {ptr: 改行, len: 1}
    (i32.store (i32.const 8754) (i32.const 8720))
    (i32.store (i32.const 8758) (i32.const 1))

    (drop (call $fd_write (i32.const 1) (i32.const 8730) (i32.const 4) (i32.const 8762)))

    ;; -vでマウントされていれば"probe.txt"へ書き込む。マウントされていない
    ;; 場合はpath_openが失敗するだけで、_start自体はエラーにしない
    ;; （実機での-v確認用。結果は無視してよい）。
    (drop (call $write_probe (i32.const 9100) (i32.const 9) (i32.const 9120) (i32.const 22)))
  )
)
