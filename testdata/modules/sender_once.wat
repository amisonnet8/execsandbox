;; フェーズ①Step5のE2E疎通確認用モジュール。
;;
;; 起動直後に固定メッセージを宛先1へ1回だけ送って終了する
;; （testing.md「一定回数送ったら終了する」の最小形）。recvは呼ばないため、
;; 受信箱を持たない送信専用サンドボックス（-nを指定しない構成）でも動く。
(module
  (import "execsandbox" "send" (func $send (param i32 i32 i32)))

  (memory (export "memory") 1)
  (data (i32.const 0) "hello-from-nodeA")

  (func (export "_start")
    (call $send (i32.const 1) (i32.const 0) (i32.const 16)))
)
