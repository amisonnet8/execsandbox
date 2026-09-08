;; フェーズ①Step3の検証用モジュール。
;;
;; sandboxパッケージの本実装（RegisterHostModule）が登録するsend/recv/max_frame
;; を、SDKを介さず直接importして確認する。個別の名前付きexportでパラメータを
;; 自由に変えたテスト（オーバーサイズ送信、バッファ不足recv等）ができるほか、
;; wazeroの既定StartFunctions（"_start"）経由の起動（本番のcmd/execsandboxが
;; 使う経路）も"_start"で検証できるようにしている。
(module
  (import "execsandbox" "send" (func $send (param i32 i32 i32)))
  (import "execsandbox" "recv" (func $recv (param i32 i32 i32 i32) (result i32)))
  (import "execsandbox" "max_frame" (func $host_max_frame (result i32)))

  (memory (export "memory") 4)

  ;; オフセット0..65536: 送信ペイロード領域（Goテストが書き込む）
  ;; オフセット65524: "_start"がrecvの戻り値を記録する場所（Goテストが読む）
  ;; オフセット65536: meta_ptr（8バイト）
  ;; オフセット65544以降: 受信バッファ

  (func (export "send_probe") (param $dest i32) (param $len i32)
    (call $send (local.get $dest) (i32.const 0) (local.get $len)))

  (func (export "recv_probe") (param $buf_cap i32) (param $timeout_ms i32) (result i32)
    (call $recv (i32.const 65536) (i32.const 65544) (local.get $buf_cap) (local.get $timeout_ms)))

  (func (export "max_frame_probe") (result i32)
    (call $host_max_frame))

  ;; _start: 1通受け取り（無限待ち）、そのままdest=1へ送り返す（エコー）。
  ;; recvの戻り値はオフセット65524に記録し、Goテストが読み出す。
  ;; recvが失敗（負値）していた場合はsendしない。
  (func (export "_start")
    (local $n i32)
    (local.set $n (call $recv (i32.const 65536) (i32.const 65544) (i32.const 65536) (i32.const -1)))
    (i32.store (i32.const 65524) (local.get $n))
    (if (i32.ge_s (local.get $n) (i32.const 0))
      (then (call $send (i32.const 1) (i32.const 65544) (local.get $n))))
  )
)
