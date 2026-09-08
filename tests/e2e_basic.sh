#!/usr/bin/env bash
# フェーズ①Step5: サンドボックス間通信の疎通確認E2E。
#
# 2つのExecSandboxプロセス（送信専用のnodeA、受信専用のnodeB）を実際に起動し、
# nodeAからnodeBへAF_UNIX経由でメッセージが届くことを確認する。
#
# nodeBは host_probe.wasm を積んでおり、その"_start"はrecv(timeout_ms=-1)で
# 1通受け取るまでブロックしたのち終了する。つまり「nodeBのプロセスが
# 自発的に終了すること」が「メッセージが届いたこと」の観測手段になる
# （送受信ともにstdoutを持たないため。.claude/rules/testing.mdの
# 「複数プロセスを扱うE2Eの注意」を参照し、PIDの確実な後始末を行う）。
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

# ソケットファイルは一時ディレクトリ配下に作る（XDG_RUNTIME_DIRを一時
# ディレクトリへ向ける）。Windows(GOOS=windows)ではXDG_RUNTIME_DIRを見ず
# %LOCALAPPDATA%を使うため、この設定はUnix系でのみ意味を持つが、
# 無条件に設定しても害はない。
export XDG_RUNTIME_DIR="$WORKDIR/xdg"
mkdir -p "$XDG_RUNTIME_DIR"

EXE_SUFFIX=""
if [ "$(go env GOOS)" = "windows" ]; then
  EXE_SUFFIX=".exe"
fi

echo "execsandbox e2e_basic: building..."
go build -o "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/cmd/execsandbox"
go build -o "$WORKDIR/stamp$EXE_SUFFIX" "$REPO_ROOT/tests/stamp"

"$WORKDIR/stamp$EXE_SUFFIX" "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/host_probe.wasm" "$WORKDIR/nodeB$EXE_SUFFIX"
"$WORKDIR/stamp$EXE_SUFFIX" "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/sender_once.wasm" "$WORKDIR/nodeA$EXE_SUFFIX"
chmod +x "$WORKDIR/nodeB$EXE_SUFFIX" "$WORKDIR/nodeA$EXE_SUFFIX"

"$WORKDIR/nodeB$EXE_SUFFIX" -n nodeB &
pid_b=$!
PIDS+=("$pid_b")

echo "execsandbox e2e_basic: waiting for nodeA -> nodeB delivery..."
delivered=0
for _ in $(seq 1 30); do
  if ! kill -0 "$pid_b" >/dev/null 2>&1; then
    delivered=1
    break
  fi
  # nodeBがまだ待ち受けを始めていないタイミングでの送信はsendの仕様上
  # 無言で捨てられる（宛先が未起動）。観測できないため、届くまで
  # リトライする。
  "$WORKDIR/nodeA$EXE_SUFFIX" -d 1=nodeB || true
  sleep 0.2
done

if [ "$delivered" -ne 1 ]; then
  echo "execsandbox e2e_basic: FAILED - nodeB did not receive nodeA's message within the timeout" >&2
  exit 1
fi

wait "$pid_b" 2>/dev/null || true
echo "execsandbox e2e_basic: OK - message delivered from nodeA to nodeB over AF_UNIX"
