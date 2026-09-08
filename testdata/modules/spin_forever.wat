;; フェーズ②Step1のスパイク検証用モジュール(経路2: 純WASMループ)。
;;
;; ホスト関数を一切呼ばず、無限ループするだけ。
;;
;; 目的：-t(実行時間制限)による強制終了が、ホスト関数呼び出しを経由しない
;; 「ゲストの計算だけが回り続けている」状態も中断できるかを検証する。
;; こちらはwazeroのRuntimeConfig.WithCloseOnContextDone(true)が挿入する
;; 周期的なチェックが担当する経路。
(module
  (func (export "_start")
    (loop $again
      (br $again)
    )
  )
)
