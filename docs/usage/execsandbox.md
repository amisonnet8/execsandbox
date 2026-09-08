日本語版: [execsandbox_ja.md](execsandbox_ja.md)

# execsandbox — launch-time options for the generated executable

The launch-time options for the executable produced by `execsandbox-build`.
Since the WASM module is already embedded, there is no argument that
specifies the module.

```
<executable> [options...] [-- arguments to the WASM module...]
```

**The contents of this page are measured values as of the completion of
Phase 4 Step 3 (all options in spec §7.1 are implemented).** The actual
`--help` output is as follows (`-V`/`-h` print `dev` on a development build
that predates the `-ldflags -X main.version=` embedding done at build
time).

```
$ execsandbox --help
Usage: execsandbox [options...] [-- args to the WASM module...]

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
```

## Option Reference

| Short | Long | Argument | Description | Default |
| :--- | :--- | :--- | :--- | :--- |
| `-n` | `--name` | `ID` | Own ID. The name other sandboxes address as a destination | none (does not receive) |
| `-d` | `--dest` | `N=ID` | Assign a destination number. Repeatable | none |
| `-e` | `--env` | `KEY=VALUE` | Environment variable. Repeatable | none |
| `-v` | `--volume` | `HOST:GUEST[:ro]` | Mount a directory. Repeatable | none (no access) |
| `-m` | `--mem-limit` | size | WASM linear memory limit | `512M` |
| `-b` | `--mailbox-limit` | integer | Mailbox capacity in messages | `1024` |
| `-f` | `--max-frame` | size | Maximum bytes per message | `1M` |
| `-l` | `--listen` | address | Listen for external connections. At most one | none (does not listen) |
| `-s` | `--stdio` | stream list | Streams to connect externally | none (all blocked) |
| `-t` | `--timeout` | duration | Execution time limit | none (unlimited) |
| `-x` | `--deny` | item list | Capabilities to deny | none (all allowed) |
| `-q` | `--quiet` | — | Suppress host-side logging | logs |
| `-h` | `--help` | — | Help | — |
| `-V` | `--version` | — | Version | — |
| `-L` | `--print-licenses` | — | Print copyright notices and full license text | — |

`-v` is for a volume, not verbose. Version is `-V`.

## Communication Settings

### `-n, --name`

Specifies your own ID. Once another sandbox names this ID with `-d`, it can
send you messages.

If omitted, the sandbox does not receive sandbox-to-sandbox messages (it
becomes send-only, or one that only accepts external connections).

### `-d, --dest`

Assigns an ID to a destination number. The number is an integer of 1 or
greater.

```
-d 1=dbcore -d 2=logger
```

The WASM module side sends by specifying only the number (`Send(1, data)`).
This launch option decides who it actually connects to.

Sending to a number that wasn't assigned isn't an error; it's silently
dropped.

Specifying the same number with `-d` twice is an error and prevents
startup (the later one does not silently overwrite the earlier one).

## Resource Limits

### `-m, --mem-limit` / `-f, --max-frame` — Size Notation

| Written as | Means |
| :--- | :--- |
| `512M` | 512 × 1024 × 1024 bytes |
| `512m` | same (case-insensitive) |
| `512Mi` | same (`Ki`/`Mi`/`Gi` also accepted) |
| `536870912` | no unit means bytes |

`K`/`M`/`G` are all interpreted as powers of 1024.

**Limits**: exceeding 4GiB (2<sup>32</sup> bytes, derived from WASM linear
memory's page-count limit) for `-m`, or 2GiB−1 bytes (because the ABI's
`max_frame()` returns an i32) for `-f`, is an error that prevents startup.
A value given to `-m` is rounded up to a multiple of 64KiB (one WASM page)
— for example, `-m 1K`'s effective value is 64KiB.

### `-b, --mailbox-limit`

The **message count** the mailbox can hold. When a new message arrives
while at capacity, that message is discarded (tail-drop). The discard is
recorded to host-side standard error.

`-b` × `-f` becomes the mailbox's memory ceiling. With the defaults, that's
1M × 1024 = 1G.

For a configuration with a high volume of small messages, you can raise the
depth while keeping the same ceiling.

```
-f 64K -b 16384    # same 1G ceiling, 16x the depth
```

### `-t, --timeout`

The execution time limit. Given as a Go-standard duration string (`30s`,
`5m`, `1h30m`, etc.). A value of `0` or less is an error ("no timeout" is
expressed by not specifying the flag at all, not by `-t 0s`).

```
-t 30s
-t 5m
```

Once the limit is reached, the WASM module is forcibly terminated. The
process's exit code becomes **124** (the same value Unix's `timeout(1)`
command uses), and the host log prints `execsandbox: execution timed out
after <duration>` (suppressible with `-q`).

**If the guest itself checks `recv`'s return value and is implemented to
exit voluntarily, it may be able to exit more gracefully (rather than being
forcibly killed) once the deadline arrives.** In that case the exit code
becomes 0 (or whatever value the guest specifies), not 124. Since `recv`
returns a timeout-equivalent value (`-1`) once the deadline arrives, an
implementation on the guest side that doesn't ignore this and instead winds
down leads to a more predictable exit.

## External Connections

### `-l, --listen`

```
-l 5432
-l 127.0.0.1:5432
-l /run/mydb.sock
-l unix:/run/mydb.sock
```

| Written as | Listens on |
| :--- | :--- |
| (omitted) | does not listen |
| `5432` | `127.0.0.1:5432` (a port-only value fills in loopback) |
| `127.0.0.1:5432` | same |
| `192.168.1.10:5432` | only the given interface |
| `:5432` | all interfaces |
| `[::1]:5432` | IPv6 loopback |
| `/run/mydb.sock` | Unix domain socket (starts with `/`) |
| `unix:/run/mydb.sock` | same (explicit `unix:` prefix) |

The host part specifies **which interface to listen on**; it does not
restrict connection sources. No filtering by source is performed, so use a
firewall or reverse proxy if needed.

At most one listener per instance. **Specifying `-l` more than once is a
startup error.** If you need more than one, stand up multiple sandboxes
that accept external connections and point them at the core.

For **a Windows path with a drive letter** (e.g. `C:\run\mydb.sock`), the
`unix:` prefix is required. Since it starts with neither `/` nor a
port-number form, omitting the prefix would attempt to parse it as
`HOST:PORT` and fail with a syntax error.

```
-l unix:C:\run\mydb.sock
```

**Stale Unix domain socket files are not cleaned up automatically.** Unlike
the listening for sandbox-to-sandbox communication done via `-n` (spec
§3.2), the path given to `-l` was explicitly chosen by the user, and it
would be dangerous to delete it on our own. If a socket file left behind by
a previous process is still there, startup fails with an error. Either
remove it manually beforehand, or manage it operationally — with systemd
socket activation, for example.

### Receive events (`recv`'s kind)

Connections accepted via `-l` funnel both sending and receiving into the
mailbox as events (spec §4.3, §5.3). The `kind` in `recv`'s `meta_ptr` takes
three values.

| `kind` | Category | `conn_id` | Payload |
| :--- | :--- | :--- | :--- |
| `1` | External connection: established | valid | none |
| `2` | External connection: data | valid | present |
| `3` | External connection: closed | valid | none |

(`kind=0` is a sandbox-to-sandbox message. See spec §5.3.)

`conn_id` is a monotonically increasing integer starting from 1, never
reused after disconnection. A guest keeps a state table keyed by `conn_id`,
creating an entry on the established event, updating it on data events, and
discarding it on the closed event.

**The disconnect event (`kind=3`) is always delivered even if the mailbox's
message-count cap (`-b`) has been reached.** Sandbox-to-sandbox messages
(`kind=0`) and the established/data events can be dropped once the cap is
reached (tail-drop), but the disconnect event alone is delivered regardless
of the cap, as an exception. This exists to prevent a leak from the guest
having no way to know when it's safe to discard its per-`conn_id` state.

### `conn_write`

`conn_write(conn_id, ptr, len)` (spec §5.4) writes back to a connection from
the guest.

| Return value | Meaning |
| :--- | :--- |
| `0` | Success |
| `-1` | Unknown `conn_id` (always `-1` if `-l` wasn't specified), or the write failed |

If the write actually fails (e.g. the peer has already disconnected), `-1`
is also returned. In that case the connection is closed internally, a
disconnect event (`kind=3`) is delivered, and from then on that `conn_id`
is treated as a genuinely "unknown `conn_id`."

**If `--timeout` is specified, that deadline also applies to waiting for a
`conn_write` to complete.** If the write doesn't finish by the deadline, it
is treated as a failure (`-1`). If `--timeout` isn't specified, the write
blocks until it completes (note that a write to a connection whose peer is
slow to receive can stall the whole guest for a long time).

## I/O and Permissions

### `-s, --stdio`

Lists the streams connected to the outside (the shell), comma-separated.

| Value | Target |
| :--- | :--- |
| `in` | standard input |
| `out` | standard output |
| `err` | standard error |
| `all` | all of the above |

```
-s out              # standard output only
-s in,out           # for an interactive module
-s all
```

A stream that isn't listed is blocked (nothing can be read, and anything
written is discarded).

File paths are not accepted. Route output using shell redirection instead.

```
./mymodule -s out > output.txt
```

### `-v, --volume`

```
-v /data:/data          # read-write
-v /etc/conf:/conf:ro   # read-only
```

Unless specified, the WASM module has no filesystem access whatsoever.

The `HOST:GUEST[:ro]` split is parsed from the end, which also allows a
Windows drive letter (as in `C:\data:/data`, where `HOST` itself contains a
`:`). `HOST` is checked for existence at startup; specifying a
nonexistent path is an error that prevents startup (the directory needs to
already exist). `GUEST` must start with `/`.

### `-x, --deny`

| Value | Blocked |
| :--- | :--- |
| `random` | random number generation |
| `time` | time retrieval |

```
-x random,time
```

Only randomness and time default to allowed (every other policy defaults to
denied). Specify this when you want to explicitly deny them.

**On the effective meaning of `-x time`**: WASI (`clock_time_get`) defines
no error path for "refuse to retrieve the time." So `-x time` is actually
realized by showing the underlying runtime's (wazero's) default fake clock
as-is — a value unrelated to real time, starting around
2022-01-01T00:00:00Z on each launch and advancing by just one millisecond
per call. A guest can only learn that time retrieval is being denied by
noticing "the value isn't real," not by an error.

## Miscellaneous

### `--` — Arguments to the WASM Module

Every argument after `--` is passed as an argument to the WASM module.

```
./mydb -e LOG=debug -d 1=core -- --verbose --level 3
                                 ^^^^^^^^^^^^^^^^^^ to the WASM module from here
```

### `-q, --quiet`

Suppresses logs emitted by ExecSandbox itself (lines starting with
`execsandbox:`). It does not affect the WASM module's own output (`-s
err`).

### `-L, --print-licenses`

```
./mydb -L
```

The generated executable contains code from both ExecSandbox itself (MIT)
and `wazero` (Apache-2.0). **Distributing this executable to a third party
carries an obligation to include both parties' copyright notices and full
license text (and, for wazero, its NOTICE) with it.** `-L,
--print-licenses` prints everything needed to satisfy that obligation to
standard output. Like `--help`/`--version`, it works regardless of whether
a WASM module is present, and is unaffected by `-q`.

The party responsible for satisfying this obligation is placed on **the
generated executable itself**, rather than on `execsandbox-build` (the
builder), because it's the output that gets distributed to third parties,
and a distributor isn't guaranteed to still have the builder on hand. If
the output itself can print the text, the obligation can be met even if the
distributor has lost access to the builder.

```
$ ./mydb -L | head -3
ExecSandbox
Licensed under the MIT License. Full text below.

```

## Host-Side Logging

The core writes logs to standard error, prefixed with `execsandbox:`. This
is kept distinct from the WASM module's own output. **Logs and error
messages are in English.**

What's mainly emitted (the actual wording):

```
execsandbox: mailbox full, dropped 3 message(s)
execsandbox: dropped 2 oversized frame(s) (max 1048576 bytes)
execsandbox: dropped 1 oversized frame(s) received from a peer (max 1048576 bytes)
execsandbox: execution timed out after 30s
```

- Message discards due to the mailbox cap (tail-drop; the first is emitted
  immediately, and after that a running total is emitted at intervals)
- Discards of messages exceeding `--max-frame` (detectable on either the
  sending or the receiving side)
- Forced termination due to `--timeout`
- Startup errors (not suppressed by `-q` — see "Startup option errors and
  exit codes" below)

Sending to an unassigned destination emits no log (it's a static state
visible just by looking at the configuration).

`-q, --quiet` suppresses everything above except startup errors (the
dynamically occurring events). It does not affect the WASM module's own
output (`-s err`).

## Startup Option Errors and Exit Codes

| Situation | Exit code | Output destination |
| :--- | :--- | :--- |
| `--help` / `-h` | 0 | standard output |
| `--version` / `-V` | 0 | standard output |
| `--print-licenses` / `-L` | 0 | standard output |
| An option error (an undefined flag, a malformed value, `-v`'s host path doesn't exist, etc.) | 2 | standard error (`execsandbox:` prefix, not suppressed even by `-q`) |
| The WASM module isn't embedded, host function registration failed, or another reason startup couldn't proceed | 1 | standard error (`execsandbox:` prefix) |
| Forcibly terminated after reaching the `--timeout` deadline | 124 | the log can be suppressed with `-q` |
| The WASM module specified its own exit code (e.g. WASI `proc_exit`) | that value, used as-is | — |
| The WASM module exited normally | 0 | — |

An actual example of an option error (exit code 2):

```
$ execsandbox -m bogus
execsandbox: invalid value "bogus" for flag -m: invalid size "bogus": must start with a number
execsandbox: run with --help for usage
```
