#!/usr/bin/env bash
# フェーズ③Step5: 外部接続(-l/--listen、conn_write)の疎通確認E2E。
#
# conn_echo.wasmを積んだサンドボックスを-lで起動し、外部のTCPクライアント
# (tests/connclient)から接続してデータを送るとそのままエコーされることを
# 確認する。TCPを使いUnixソケットパスを引数に渡さないことで、Git Bash/MSYS
# (windows-latest)のパス自動変換(.claude/rules/testing.md)を回避する。
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

EXE_SUFFIX=""
if [ "$(go env GOOS)" = "windows" ]; then
  EXE_SUFFIX=".exe"
fi

PORT=18902

echo "execsandbox e2e_conn: building..."
go build -o "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/cmd/execsandbox"
go build -o "$WORKDIR/stamp$EXE_SUFFIX" "$REPO_ROOT/tests/stamp"
go build -o "$WORKDIR/connclient$EXE_SUFFIX" "$REPO_ROOT/tests/connclient"

"$WORKDIR/stamp$EXE_SUFFIX" "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/conn_echo.wasm" "$WORKDIR/echo$EXE_SUFFIX"
chmod +x "$WORKDIR/echo$EXE_SUFFIX"

# -tを付けておくと、万一connclientとの疎通に失敗してもサンドボックス自体は
# 期限内に強制終了する(孤児プロセス化しにくくする安全網。trapによるkillとは
# 独立に用意する。.claude/rules/testing.md「複数プロセスを扱うE2Eの注意」)。
"$WORKDIR/echo$EXE_SUFFIX" -l "$PORT" -t 20s &
pid_echo=$!
PIDS+=("$pid_echo")

echo "execsandbox e2e_conn: waiting for the listener and checking the echo..."
ok=0
for _ in $(seq 1 30); do
  if "$WORKDIR/connclient$EXE_SUFFIX" tcp "127.0.0.1:$PORT" "hello from connclient"; then
    ok=1
    break
  fi
  sleep 0.2
done

if [ "$ok" -ne 1 ]; then
  echo "execsandbox e2e_conn: FAILED - did not receive a correct echo within the timeout" >&2
  exit 1
fi

echo "execsandbox e2e_conn: OK - external connection accepted and echoed via -l/conn_write"
