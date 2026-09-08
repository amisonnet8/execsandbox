;; フェーズ②Step1のスパイク検証用モジュール(経路1: ホスト関数内ブロック)。
;;
;; "_start"がrecv(timeout_ms=-1)を無限に呼び続けるだけ。メッセージは一切
;; 届かない前提で、ホスト関数呼び出しの中でブロックし続けている状態を作る。
;;
;; 目的：-t(実行時間制限)でプロセス全体を強制終了する際、wazeroの
;; WithCloseOnContextDone(true)がこの「ホスト関数内でブロック中のゲスト」を
;; 安全に中断できるかを検証する(PLAN.md保留事項「recvのタイムアウト実装方式」)。
(module
  (import "execsandbox" "recv" (func $recv (param i32 i32 i32 i32) (result i32)))

  (memory (export "memory") 1)
  ;; オフセット0: meta_ptr(8バイト)、オフセット8: 受信バッファ(256バイト)

  (func (export "_start")
    (loop $again
      (drop (call $recv (i32.const 0) (i32.const 8) (i32.const 256) (i32.const -1)))
      (br $again)
    )
  )
)
