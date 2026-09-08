#!/usr/bin/env bash
# フェーズ④Step4: ビルダー(cmd/execsandbox-build)のE2E確認。
#
# 実際にビルダーをビルドし、検証用wasmを自環境向け(--target省略)で
# スタンプして、生成した実行ファイルが実際に動くことを確認する
# (tests/e2e_basic.shと同じ「送受信の疎通」を観測手段にする)。
#
# クロスターゲット(例:linuxランナー上でのwindows/amd64向け生成)は
# このランナー上では実行できないため、生成されたバイト列が
# 「embedされたベースバイナリ + wasm + フッター」の結合として正しい
# ことをバイト比較で検証する。3OSのCIマトリクス(ubuntu-latest=
# linux/amd64、macos-latest=darwin/arm64、windows-latest=windows/amd64が
# 典型)により、6環境のうち3環境は実行確認、残り3環境もこの
# バイト比較で担保される。
#
# 前提: cmd/execsandbox-build/basebinaries/ (6環境分のcmd/execsandbox)が
# 事前に用意されていること(Makefileの`cross-base`、またはCIの
# "build base binaries"ステップ)。このスクリプト自身はそれを行わない。
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

WORKDIR=$(mktemp -d)
PIDS=()
cleanup() {
  if [ "${#PIDS[@]}" -gt 0 ]; then
    for pid in "${PIDS[@]}"; do
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
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

echo "execsandbox e2e_builder: building execsandbox-build..."
go build -o "$WORKDIR/execsandbox-build$EXE_SUFFIX" "$REPO_ROOT/cmd/execsandbox-build"

echo "execsandbox e2e_builder: stamping for this machine's platform (--target omitted)..."
"$WORKDIR/execsandbox-build$EXE_SUFFIX" -o "$WORKDIR/nodeB$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/host_probe.wasm"
"$WORKDIR/execsandbox-build$EXE_SUFFIX" -o "$WORKDIR/nodeA$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/sender_once.wasm"
chmod +x "$WORKDIR/nodeB$EXE_SUFFIX" "$WORKDIR/nodeA$EXE_SUFFIX"

echo "execsandbox e2e_builder: checking that the builder-generated binaries actually run..."
"$WORKDIR/nodeB$EXE_SUFFIX" -n nodeB &
pid_b=$!
PIDS+=("$pid_b")

delivered=0
for _ in $(seq 1 30); do
  if ! kill -0 "$pid_b" >/dev/null 2>&1; then
    delivered=1
    break
  fi
  # nodeBがまだ待ち受けを始めていないタイミングでの送信は無言で捨てられる
  # ため、届くまでリトライする(tests/e2e_basic.shと同じ理由)。
  "$WORKDIR/nodeA$EXE_SUFFIX" -d 1=nodeB || true
  sleep 0.2
done

if [ "$delivered" -ne 1 ]; then
  echo "execsandbox e2e_builder: FAILED - nodeB did not receive nodeA's message within the timeout" >&2
  exit 1
fi
wait "$pid_b" 2>/dev/null || true

echo "execsandbox e2e_builder: OK - builder-generated binaries actually ran and communicated"

echo "execsandbox e2e_builder: checking cross-target output byte-for-byte against the embedded base binaries..."
wasm_path="$REPO_ROOT/testdata/modules/host_probe.wasm"
wasm_size=$(wc -c <"$wasm_path")

for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  goos="${t%/*}"
  goarch="${t#*/}"

  base_name="execsandbox_${goos}_${goarch}"
  if [ "$goos" = "windows" ]; then
    base_name="${base_name}.exe"
  fi
  base_path="$REPO_ROOT/cmd/execsandbox-build/basebinaries/$base_name"
  if [ ! -f "$base_path" ]; then
    echo "execsandbox e2e_builder: FAILED - expected embedded base binary not found: $base_path" >&2
    exit 1
  fi
  base_size=$(wc -c <"$base_path")

  out_path="$WORKDIR/cross_${goos}_${goarch}"
  "$WORKDIR/execsandbox-build$EXE_SUFFIX" -o "$out_path" --target "$t" "$wasm_path"
  if [ "$goos" = "windows" ]; then
    out_path="${out_path}.exe"
  fi

  expected_size=$((base_size + wasm_size + 32))
  actual_size=$(wc -c <"$out_path")
  if [ "$actual_size" -ne "$expected_size" ]; then
    echo "execsandbox e2e_builder: FAILED - $t output size = $actual_size, want $expected_size (base=$base_size + wasm=$wasm_size + footer=32)" >&2
    exit 1
  fi

  if ! cmp -s <(head -c "$base_size" "$out_path") "$base_path"; then
    echo "execsandbox e2e_builder: FAILED - $t output does not start with the embedded base binary" >&2
    exit 1
  fi

  if ! cmp -s <(tail -c +"$((base_size + 1))" "$out_path" | head -c "$wasm_size") "$wasm_path"; then
    echo "execsandbox e2e_builder: FAILED - $t output's WASM section does not match the input .wasm" >&2
    exit 1
  fi
done

echo "execsandbox e2e_builder: OK - all 6 targets produce byte-correct output"
