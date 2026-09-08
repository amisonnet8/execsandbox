package main

// ヘルプ文言(仕様書§7.1)。fs.PrintDefaults()は短形・長形が別エントリで
// 二重に並び表形式にできないため使わず、手書きにする。

import (
	"fmt"
	"io"
)

const usageText = `Usage: execsandbox [options...] [-- args to the WASM module...]

Options:
  -n, --name ID                  own ID for sandbox-to-sandbox messaging
                                  (default: none, does not receive)
  -d, --dest N=ID                 assign a destination number to an ID
                                  (repeatable)
  -e, --env KEY=VALUE             environment variable (repeatable)
  -v, --volume HOST:GUEST[:ro]    mount a directory (repeatable)
                                  (default: none, no filesystem access)
  -m, --mem-limit SIZE            WASM linear memory limit (default: 512M)
  -b, --mailbox-limit N           mailbox capacity in messages (default: 1024)
  -f, --max-frame SIZE            maximum message size (default: 1M)
  -l, --listen ADDR               external connection listen address
                                  (default: none, does not listen)
  -s, --stdio LIST                streams to connect to the shell:
                                  in,out,err,all (default: none, all blocked)
  -t, --timeout DURATION          execution time limit, e.g. 30s, 5m
                                  (default: none, unlimited)
  -x, --deny LIST                 capabilities to deny: random,time
                                  (default: none, both allowed)
  -q, --quiet                     suppress host-side logging
  -h, --help                      show this help message
  -V, --version                   print the version
  -L, --print-licenses            print third-party license notices and exit

Sizes (-m, -f) accept K/M/G suffixes (1024-based), case-insensitive, with an
optional "i" (e.g. 512M, 512Mi). A number with no suffix is bytes.

Everything after "--" is passed to the WASM module as its arguments.
`

func writeUsage(w io.Writer) {
	fmt.Fprint(w, usageText)
}
