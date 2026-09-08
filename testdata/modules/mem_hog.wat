;; フェーズ②Step5の検証用モジュール。
;;
;; メモリの最大値を宣言せず(モジュール側の上限で頭打ちにならないように)、
;; memory.growが失敗(-1)するまで1ページずつ伸ばし続ける。到達した実際の
;; ページ数(=ホスト側のWithMemoryLimitPagesで与えた上限)をexportで返す
;; ことで、-m/--mem-limitの効果を直接確認する。
(module
  (memory (export "memory") 1)

  (func (export "grow_until_fail") (result i32)
    (loop $again
      (if (i32.ne (memory.grow (i32.const 1)) (i32.const -1))
        (then (br $again))))
    (memory.size))
)
