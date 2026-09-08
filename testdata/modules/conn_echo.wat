;; フェーズ③Step4検証用モジュール。
;;
;; recv(timeout_ms=-1)を無限ループで呼び続け、kind=2(外部接続・データ)の
;; イベントだけをconn_write(仕様書§5.4)で書き戻す(エコー)。kind=1(確立)・
;; kind=3(切断)、および将来追加されうる未知のkind、recvの失敗(タイムアウト・
;; バッファ不足、負値)はいずれも無視してループを継続する(仕様書§5.3
;; 「未知のkindは無視する」を検証用モジュール自身でも実践する)。
;;
;; -t(--timeout)や、テスト側からのcontext取り消しでrecvがブロックを解け
;; -1を返しても、ループ自体は止まらずrecvを呼び直し続ける(blocker.watと
;; 同じ経路)。最終的にはwazeroのWithCloseOnContextDoneによる強制終了に頼る。
(module
  (import "execsandbox" "recv" (func $recv (param i32 i32 i32 i32) (result i32)))
  (import "execsandbox" "conn_write" (func $conn_write (param i32 i32 i32) (result i32)))

  (memory (export "memory") 2)
  ;; オフセット0..7: meta_ptr (kind: u32 LE, conn_id: u32 LE)
  ;; オフセット8..: 受信バッファ(64KiB)

  (func (export "_start")
    (local $n i32)
    (loop $again
      (local.set $n (call $recv (i32.const 0) (i32.const 8) (i32.const 65536) (i32.const -1)))
      (if (i32.ge_s (local.get $n) (i32.const 0))
        (then
          (if (i32.eq (i32.load (i32.const 0)) (i32.const 2))
            (then
              (drop (call $conn_write (i32.load (i32.const 4)) (i32.const 8) (local.get $n)))
            )
          )
        )
      )
      (br $again)
    )
  )
)
