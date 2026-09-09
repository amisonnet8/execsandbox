#!/usr/bin/env bash
# フェーズ②Step8: ポリシー系オプション(-s/-e/"--"/-q)とCLIの基本動作
# (--help/-V/不正オプション)のE2E確認。
#
# 単一プロセスで完結する検証のみを集める（複数プロセスの疎通は
# tests/e2e_basic.sh、-tの強制終了は tests/e2e_timeout.sh が担当）。
set -euo pipefail

# wasi_probe.wasmの"_start"は、-a/--allowの実機確認用にrandom_get/
# clock_time_getの生バイト(乱数・時刻)を無条件にstdoutの末尾へ書き出す
# (testdata/modules/wasi_probe.wat)。macOSのtr/grep(BSD版)はロケールに
# 応じて入力をUTF-8として妥当性検証するため、この非ASCIIな乱数バイト列に
# 遭遇すると"Illegal byte sequence"で落ちる(GNU版のtr/grepでは起きない
# 差異。手元Linuxでは再現せずmacos-latestランナーでのみ顕在化した)。
# LC_ALL=Cでバイト列をそのまま扱わせることで回避する
# (.claude/rules/testing.md「クロスプラットフォームCIの落とし穴」参照)。
export LC_ALL=C

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

WORKDIR=$(mktemp -d)
cleanup() {
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

export XDG_RUNTIME_DIR="$WORKDIR/xdg"
mkdir -p "$XDG_RUNTIME_DIR"

EXE_SUFFIX=""
if [ "$(go env GOOS)" = "windows" ]; then
  EXE_SUFFIX=".exe"
fi

echo "execsandbox e2e_policy: building..."
go build -o "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/cmd/execsandbox"
go build -o "$WORKDIR/stamp$EXE_SUFFIX" "$REPO_ROOT/tests/stamp"

"$WORKDIR/stamp$EXE_SUFFIX" "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/wasi_probe.wasm" "$WORKDIR/probe$EXE_SUFFIX"
"$WORKDIR/stamp$EXE_SUFFIX" "$WORKDIR/base$EXE_SUFFIX" "$REPO_ROOT/testdata/modules/sender_once.wasm" "$WORKDIR/sender$EXE_SUFFIX"
chmod +x "$WORKDIR/probe$EXE_SUFFIX" "$WORKDIR/sender$EXE_SUFFIX"

# --- -s out / -e / "--" 以降の引数 ---
echo "execsandbox e2e_policy: checking -s out / -e / -- args..."
out=$("$WORKDIR/probe$EXE_SUFFIX" -s out -e KEY=VALUE -- arg1 arg2 | tr '\0' '\n')
if ! printf '%s' "$out" | grep -qx "execsandbox"; then
  echo "execsandbox e2e_policy: FAILED - argv[0] 'execsandbox' not found in output" >&2
  exit 1
fi
if ! printf '%s' "$out" | grep -qx "arg1"; then
  echo "execsandbox e2e_policy: FAILED - 'arg1' not found in output" >&2
  exit 1
fi
if ! printf '%s' "$out" | grep -qx "arg2"; then
  echo "execsandbox e2e_policy: FAILED - 'arg2' not found in output" >&2
  exit 1
fi
if ! printf '%s' "$out" | grep -qx "KEY=VALUE"; then
  echo "execsandbox e2e_policy: FAILED - env var 'KEY=VALUE' not found in output" >&2
  exit 1
fi

# --- -s未指定なら標準出力は空 ---
out_noflag=$("$WORKDIR/probe$EXE_SUFFIX" -e KEY=VALUE -- arg1 arg2)
if [ -n "$out_noflag" ]; then
  echo "execsandbox e2e_policy: FAILED - stdout must be empty without -s out, got: $out_noflag" >&2
  exit 1
fi

# --- --help / -V ---
echo "execsandbox e2e_policy: checking --help / -V..."
set +e
help_out=$("$WORKDIR/probe$EXE_SUFFIX" --help)
help_code=$?
set -e
if [ "$help_code" -ne 0 ] || ! printf '%s' "$help_out" | grep -q "^Usage: execsandbox"; then
  echo "execsandbox e2e_policy: FAILED - --help exit=$help_code output=$help_out" >&2
  exit 1
fi

set +e
version_out=$("$WORKDIR/probe$EXE_SUFFIX" -V)
version_code=$?
set -e
if [ "$version_code" -ne 0 ] || ! printf '%s' "$version_out" | grep -q "^execsandbox "; then
  echo "execsandbox e2e_policy: FAILED - -V exit=$version_code output=$version_out" >&2
  exit 1
fi

# --- 不正オプション ---
echo "execsandbox e2e_policy: checking an invalid option..."
set +e
bad_err=$("$WORKDIR/probe$EXE_SUFFIX" --bogus 2>&1 >/dev/null)
bad_code=$?
set -e
if [ "$bad_code" -ne 2 ] || ! printf '%s' "$bad_err" | grep -q "^execsandbox:"; then
  echo "execsandbox e2e_policy: FAILED - --bogus exit=$bad_code stderr=$bad_err" >&2
  exit 1
fi

# --- -q: フレーム長超過のログ抑制 ---
# sender_once.wasmは固定16バイトを送る（testdata/modules/sender_once.wat）。
# -f 8 で上限を下回らせ、nodeBの起動有無に関わらず送信側で確実に
# oversizedログが出る状況を作る（send()はDestTable.Sendへ渡す前に
# MaxFrameを検査するため、宛先が実在しなくても再現できる）。
echo "execsandbox e2e_policy: checking -q suppresses the oversized-frame log..."
set +e
loud_err=$("$WORKDIR/sender$EXE_SUFFIX" -f 8 -d 1=nowhere 2>&1 >/dev/null)
set -e
if ! printf '%s' "$loud_err" | grep -q "oversized frame"; then
  echo "execsandbox e2e_policy: FAILED - expected an oversized-frame log without -q, got: $loud_err" >&2
  exit 1
fi

set +e
quiet_err=$("$WORKDIR/sender$EXE_SUFFIX" -f 8 -q -d 1=nowhere 2>&1 >/dev/null)
set -e
if [ -n "$quiet_err" ]; then
  echo "execsandbox e2e_policy: FAILED - -q must suppress the oversized-frame log, got: $quiet_err" >&2
  exit 1
fi

echo "execsandbox e2e_policy: OK"
