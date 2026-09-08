;; フェーズ①Step1のスパイク検証用モジュール。
;;
;; 目的：仕様書§5のABI（send/recvのシグネチャ、meta_ptrの8バイトレイアウト）が
;; wazero上で成立するかを確認する。SDKを介さず、ホスト関数を直接importする。
;;
;; 手順：
;;   1. send(dest=1, ptr, len) で "hello" を送る
;;      → ホストがゲスト線形メモリを正しく読めるかの検証
;;   2. recv(meta_ptr, buf_ptr, buf_cap, timeout_ms=-1) を呼ぶ
;;      → ホストがゲスト線形メモリ（メタデータ8バイト＋ペイロード）へ
;;        正しく書き込めるかの検証
;;   3. recvの戻り値（ペイロード長）をそのまま "run" の戻り値として返す
;;      → Go側のテストがホスト関数呼び出し経由で結果を検証する
(module
  (import "execsandbox" "send" (func $send (param i32 i32 i32)))
  (import "execsandbox" "recv" (func $recv (param i32 i32 i32 i32) (result i32)))

  (memory (export "memory") 1)

  ;; オフセット0: send で送る "hello"（5バイト）
  (data (i32.const 0) "hello")

  ;; オフセット64: recv が書き込む meta_ptr（8バイト固定）
  ;; オフセット128: recv が書き込む buf_ptr（256バイト確保）
  (func (export "run") (result i32)
    (call $send (i32.const 1) (i32.const 0) (i32.const 5))
    (call $recv (i32.const 64) (i32.const 128) (i32.const 256) (i32.const -1))
  )
)
