#!/usr/bin/env bash
# フェーズ②Step7: -t/--timeoutのE2E確認。
#
# blocker.wasm（フェーズ②Step1のスパイク検証で使ったもの）の"_start"は
# recv(timeout_ms=-1)の呼び出しを結果を見ずに無限ループし続ける。recv自体は
# contextのキャンセルを見て即座に-1を返すが、blocker側はその-1を無視して
# 即座にrecvを呼び直すだけなので、緩やかに終了することはない。
# WithCloseOnContextDone（-t指定時のみ有効化、sandbox/policy.go）による
# 強制終了に頼るしかない経路であり、-tが本当にプロセスを強制終了できる
# ことのE2E確認として適している。
#
# なお同じ検証をhost_probe.wasm（tests/e2e_basic.sh）で行うと、recvの
# 単発呼び出しがcontextキャンセルで-1を返した時点で"_start"が正常終了して
# しまい（-tで強制終了される前に自発的に終わる）、ExitCodeDeadlineExceeded
# の経路を通らない。これは望ましい違い（強制終了されるより自発的に
# 終わる方が良い）ではあるが、-tの強制終了機構そのものの検証には
# blocker.wasmが要る。
#
# -nも付けて起動することで、強制終了後にソケットファイルが残らないことも
# 併せて確認する（.claude/rules/testing.md「複数プロセスを扱うE2Eの注意」に
# 従いPIDの確実な後始末を行う）。
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

WORKDIR=$(mktemp -d)
PIDS=()
cleanup() {
  if [ "${#PIDS[@]}" -gt 0 ]; then
    for pid in "${PIDS[@]}"; do
      kill "$pid" >/dev/null 2>&1 || true
    done
  fi
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

export XDG_RUNTIME_DIR="$WORKDIR/xdg"
mkdir -p "$XDG_RUNTIME_DIR"

EXE_SUFFIX=""
if [ "$(go env GOOS)" = "windows" ]; then
  EXE_SUFFIX=".exe"
fi

echo "execsandbox e2e_timeout: building..."
go build -o "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/cmd/execsandbox"
go build -o "$WORKDIR/stamp$EXE_SUFFIX" "$REPO_ROOT/tests/stamp"

"$WORKDIR/stamp$EXE_SUFFIX" "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/blocker.wasm" "$WORKDIR/nodeB$EXE_SUFFIX"
chmod +x "$WORKDIR/nodeB$EXE_SUFFIX"

SOCK="$XDG_RUNTIME_DIR/execsandbox/nodeB.sock"

echo "execsandbox e2e_timeout: starting nodeB (blocker.wasm) with -t 1s (it must be force-killed by the timeout)..."
started=$(date +%s)
"$WORKDIR/nodeB$EXE_SUFFIX" -n nodeB -t 1s &
pid_b=$!
PIDS+=("$pid_b")

set +e
wait "$pid_b"
exit_code=$?
set -e
elapsed=$(( $(date +%s) - started ))

if [ "$exit_code" -ne 124 ]; then
  echo "execsandbox e2e_timeout: FAILED - exit code = $exit_code, want 124" >&2
  exit 1
fi

if [ "$elapsed" -gt 5 ]; then
  echo "execsandbox e2e_timeout: FAILED - took ${elapsed}s to exit, want well under 5s for a 1s timeout" >&2
  exit 1
fi

if [ -S "$SOCK" ] || [ -e "$SOCK" ]; then
  echo "execsandbox e2e_timeout: FAILED - socket file $SOCK still exists after the process exited" >&2
  exit 1
fi

echo "execsandbox e2e_timeout: OK - timed out after ${elapsed}s with exit code 124, socket cleaned up"
