;; フェーズ②Step4の検証用モジュール。
;;
;; SDKを介さず"wasi_snapshot_preview1"のargs_get/environ_get/fd_writeを
;; 直接importし、-e(環境変数)・"--"以降の引数・-s out(stdout配線)が
;; ゲストへ実際に届くことを確認する（.claude/rules/testing.md「生のABIを
;; 直接叩くこと」）。
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
;;
;; _start: argv_bufと改行とenviron_bufと改行を、1回のfd_write（iovec 4本）で
;; fd=1（stdout）へまとめて書き出す。-s outが無指定の場合、wazeroの既定
;; Stdout（io.Discard）へ書かれるだけでエラーにはならない（何もしなければ
;; 何もできない、という既定遮断の挙動そのものを確認する）。
(module
  (import "wasi_snapshot_preview1" "args_sizes_get" (func $args_sizes_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "args_get" (func $args_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "environ_sizes_get" (func $environ_sizes_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "environ_get" (func $environ_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "fd_write" (func $fd_write (param i32 i32 i32 i32) (result i32)))

  (memory (export "memory") 1)
  (data (i32.const 8720) "\0a") ;; 改行1バイト

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
  )
)
