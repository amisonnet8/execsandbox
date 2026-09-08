package main

// 起動時CLIオプションのパース(仕様書§7)。
//
// フェーズ②で仕様書§7.1の全オプションのパース・検証・wazeroへの配線が完了した。
// フェーズ③Step2時点では-l/--listenを追加した。書式(仕様書§7.4)の検証は
// sandbox.ParseListenAddressで行い、2回以上の指定はエラーにする(§4.1
// 「1インスタンスにつき1つのみ」)。待ち受けの開始自体はフェーズ③Step3で
// main.goに配線する。

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amisonnet8/execsandbox/sandbox"
)

// 仕様書§7.1の既定値。
const (
	defaultMemLimit     = 512 * 1024 * 1024 // 512M
	defaultMailboxLimit = 1024
	defaultMaxFrame     = 1 << 20 // 1M
)

type envVar struct {
	Key, Value string
}

type volumeMount struct {
	Host, Guest string
	ReadOnly    bool
}

type stdioSet struct {
	In, Out, Err bool
}

type denySet struct {
	Random, Time bool
}

// options はパース済みの起動時オプション一式。
type options struct {
	name         string
	dest         destAssignments
	env          []envVar
	volumes      []volumeMount
	memLimit     int64
	mailboxLimit int
	maxFrame     int64
	stdio        stdioSet
	timeout      time.Duration // 0はタイムアウトなし(既定)
	deny         denySet
	listen       string // 空文字列は待ち受けなし(既定)
	quiet        bool
	help         bool
	version      bool
	guestArgs    []string // "--"以降
}

// parseArgs はargs(通常os.Args[1:])から起動時オプションを組み立てる。
// -h/--helpまたは-V/--versionが指定された場合、他の値の検証は行わずに
// optsを返す(呼び出し側でhelp/versionを先にチェックする)。
func parseArgs(args []string) (*options, error) {
	fs := flag.NewFlagSet("execsandbox", flag.ContinueOnError)
	// エラー・使い方の出力はflagパッケージに任せず、呼び出し側で
	// execsandbox:接頭辞付きの英語メッセージとして統一する
	// (.claude/rules/cli-output.md)。
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	opts := &options{
		dest:         make(destAssignments),
		memLimit:     defaultMemLimit,
		mailboxLimit: defaultMailboxLimit,
		maxFrame:     defaultMaxFrame,
	}

	fs.StringVar(&opts.name, "n", "", "own ID for sandbox-to-sandbox messaging")
	fs.StringVar(&opts.name, "name", "", "own ID for sandbox-to-sandbox messaging")

	fs.Var(opts.dest, "d", "assign a destination number to an ID, N=ID (repeatable)")
	fs.Var(opts.dest, "dest", "assign a destination number to an ID, N=ID (repeatable)")

	fs.Var(envList{&opts.env}, "e", "environment variable, KEY=VALUE (repeatable)")
	fs.Var(envList{&opts.env}, "env", "environment variable, KEY=VALUE (repeatable)")

	fs.Var(volumeList{&opts.volumes}, "v", "mount a directory, HOST:GUEST[:ro] (repeatable)")
	fs.Var(volumeList{&opts.volumes}, "volume", "mount a directory, HOST:GUEST[:ro] (repeatable)")

	fs.Var((*sizeValue)(&opts.memLimit), "m", "WASM linear memory limit (e.g. 512M)")
	fs.Var((*sizeValue)(&opts.memLimit), "mem-limit", "WASM linear memory limit (e.g. 512M)")

	fs.IntVar(&opts.mailboxLimit, "b", defaultMailboxLimit, "mailbox capacity in messages")
	fs.IntVar(&opts.mailboxLimit, "mailbox-limit", defaultMailboxLimit, "mailbox capacity in messages")

	fs.Var((*sizeValue)(&opts.maxFrame), "f", "maximum message size (e.g. 1M)")
	fs.Var((*sizeValue)(&opts.maxFrame), "max-frame", "maximum message size (e.g. 1M)")

	fs.Var(&opts.stdio, "s", "streams to connect to the shell: in,out,err,all")
	fs.Var(&opts.stdio, "stdio", "streams to connect to the shell: in,out,err,all")

	var listenSeen bool
	fs.Var(listenValue{&opts.listen, &listenSeen}, "l", "external connection listen address (only one allowed)")
	fs.Var(listenValue{&opts.listen, &listenSeen}, "listen", "external connection listen address (only one allowed)")

	fs.Var((*durationValue)(&opts.timeout), "t", "execution time limit (e.g. 30s, 5m)")
	fs.Var((*durationValue)(&opts.timeout), "timeout", "execution time limit (e.g. 30s, 5m)")

	fs.Var(&opts.deny, "x", "capabilities to deny: random,time")
	fs.Var(&opts.deny, "deny", "capabilities to deny: random,time")

	fs.BoolVar(&opts.quiet, "q", false, "suppress host-side logging")
	fs.BoolVar(&opts.quiet, "quiet", false, "suppress host-side logging")

	fs.BoolVar(&opts.help, "h", false, "show this help message")
	fs.BoolVar(&opts.help, "help", false, "show this help message")

	fs.BoolVar(&opts.version, "V", false, "print the version")
	fs.BoolVar(&opts.version, "version", false, "print the version")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if opts.help || opts.version {
		return opts, nil
	}

	if err := opts.validate(); err != nil {
		return nil, err
	}

	opts.guestArgs = fs.Args()
	return opts, nil
}

func (o *options) validate() error {
	if o.memLimit <= 0 {
		return fmt.Errorf("invalid -m/--mem-limit: must be greater than 0")
	}
	if o.mailboxLimit < 1 {
		return fmt.Errorf("invalid -b/--mailbox-limit: must be at least 1")
	}
	if o.maxFrame <= 0 {
		return fmt.Errorf("invalid -f/--max-frame: must be greater than 0")
	}
	// max_frame()はi32を返し(仕様書§5.5)、recvの不足バッファ表現も
	// -(len)-1をi32に収める(.claude/rules/abi-compatibility.md)ため、
	// この範囲に収まらない値は起動時に弾く。
	if o.maxFrame > math.MaxInt32 {
		return fmt.Errorf("invalid -f/--max-frame: must not exceed %d bytes (2G-1)", int32(math.MaxInt32))
	}
	for _, v := range o.volumes {
		if _, err := os.Stat(v.Host); err != nil {
			return fmt.Errorf("invalid -v/--volume: host path %q: %w", v.Host, err)
		}
	}
	return nil
}

// --- -d, --dest: N=ID (繰り返し可、重複番号はエラー) ---

type destAssignments map[uint32]string

func (d destAssignments) String() string {
	return "" // 複数指定できるため既定値表示は使わない
}

func (d destAssignments) Set(s string) error {
	n, id, ok := strings.Cut(s, "=")
	if !ok || n == "" || id == "" {
		return fmt.Errorf("invalid -d/--dest value %q, want N=ID", s)
	}
	num, err := strconv.ParseUint(n, 10, 32)
	if err != nil || num == 0 {
		return fmt.Errorf("invalid destination number %q in %q, want a positive integer", n, s)
	}
	if _, exists := d[uint32(num)]; exists {
		return fmt.Errorf("destination number %d is already assigned", num)
	}
	d[uint32(num)] = id
	return nil
}

// --- -e, --env: KEY=VALUE (繰り返し可) ---

type envList struct {
	vars *[]envVar
}

func (e envList) String() string { return "" }

func (e envList) Set(s string) error {
	key, value, ok := strings.Cut(s, "=")
	if !ok || key == "" {
		return fmt.Errorf("invalid -e/--env value %q, want KEY=VALUE", s)
	}
	*e.vars = append(*e.vars, envVar{Key: key, Value: value})
	return nil
}

// --- -v, --volume: HOST:GUEST[:ro] (繰り返し可) ---

type volumeList struct {
	mounts *[]volumeMount
}

func (v volumeList) String() string { return "" }

func (v volumeList) Set(s string) error {
	m, err := parseVolume(s)
	if err != nil {
		return err
	}
	*v.mounts = append(*v.mounts, m)
	return nil
}

// parseVolume は"HOST:GUEST[:ro]"を解釈する。
//
// Windowsのドライブレター(例: "C:\data")がホストパスの先頭付近に":"を
// 含むため、最初の":"で単純に分割すると壊れる。ゲストパスはWASI仮想
// ファイルシステム上のパスであり":"を含み得ないという前提のもと、
// 「末尾から見て最後の":"」で分割する(実装はGOOSに依存しない純関数にして
// あるため、Linux上でもWindows形式の入力を検証できる。
// sandbox/address.goのresolveSocketPath(goos, ...)と同じ考え方)。
func parseVolume(s string) (volumeMount, error) {
	readOnly := false
	rest := s
	if strings.HasSuffix(rest, ":ro") {
		readOnly = true
		rest = strings.TrimSuffix(rest, ":ro")
	}

	idx := strings.LastIndex(rest, ":")
	if idx <= 0 || idx == len(rest)-1 {
		return volumeMount{}, fmt.Errorf("invalid -v/--volume value %q, want HOST:GUEST[:ro]", s)
	}
	host := rest[:idx]
	guest := rest[idx+1:]
	if !strings.HasPrefix(guest, "/") {
		return volumeMount{}, fmt.Errorf("invalid -v/--volume value %q: guest path %q must start with \"/\"", s, guest)
	}
	return volumeMount{Host: host, Guest: guest, ReadOnly: readOnly}, nil
}

// --- -m/-f: サイズ表記(仕様書§7.3) ---

type sizeValue int64

func (s *sizeValue) String() string {
	return strconv.FormatInt(int64(*s), 10)
}

func (s *sizeValue) Set(v string) error {
	n, err := parseSize(v)
	if err != nil {
		return err
	}
	*s = sizeValue(n)
	return nil
}

// parseSize は仕様書§7.3のサイズ表記をバイト数へ変換する。
// K/M/Gは1024の冪、大文字小文字を区別せず、Ki/Mi/Giも同値として受理する。
// 接尾辞なしはバイト数そのもの。
func parseSize(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid size %q: empty", s)
	}

	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("invalid size %q: must start with a number", s)
	}

	n, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %v", s, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid size %q: must not be negative", s)
	}

	var mult int64
	switch strings.ToLower(s[i:]) {
	case "":
		mult = 1
	case "k", "ki":
		mult = 1 << 10
	case "m", "mi":
		mult = 1 << 20
	case "g", "gi":
		mult = 1 << 30
	default:
		return 0, fmt.Errorf("invalid size %q: unknown suffix %q, want K/M/G optionally with an \"i\" (e.g. 512M, 512Mi)", s, s[i:])
	}

	if n != 0 && n > math.MaxInt64/mult {
		return 0, fmt.Errorf("invalid size %q: too large", s)
	}
	return n * mult, nil
}

// --- -s, --stdio: カンマ区切り(in,out,err,all) ---

func (s *stdioSet) String() string { return "" }

func (s *stdioSet) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		switch v {
		case "in":
			s.In = true
		case "out":
			s.Out = true
		case "err":
			s.Err = true
		case "all":
			s.In, s.Out, s.Err = true, true, true
		default:
			return fmt.Errorf("invalid -s/--stdio value %q, want a comma-separated list of in,out,err,all", v)
		}
	}
	return nil
}

// --- -x, --deny: カンマ区切り(random,time) ---

func (d *denySet) String() string { return "" }

func (d *denySet) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		switch v {
		case "random":
			d.Random = true
		case "time":
			d.Time = true
		default:
			return fmt.Errorf("invalid -x/--deny value %q, want a comma-separated list of random,time", v)
		}
	}
	return nil
}

// --- -l, --listen: 外部接続の待ち受けアドレス(仕様書§7.4、1インスタンス
// につき1つのみ) ---

type listenValue struct {
	target *string
	seen   *bool
}

func (l listenValue) String() string { return "" }

func (l listenValue) Set(s string) error {
	if *l.seen {
		return fmt.Errorf("invalid -l/--listen: specified more than once, only one listener is allowed")
	}
	if _, _, err := sandbox.ParseListenAddress(s); err != nil {
		return err
	}
	*l.target = s
	*l.seen = true
	return nil
}

// --- -t, --timeout: Go形式の期間文字列 ---

type durationValue time.Duration

func (d *durationValue) String() string {
	return time.Duration(*d).String()
}

func (d *durationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid -t/--timeout value %q: want a Go-style duration such as \"30s\", \"5m\", \"1h30m\"", s)
	}
	if v <= 0 {
		return fmt.Errorf("invalid -t/--timeout value %q: must be greater than 0", s)
	}
	*d = durationValue(v)
	return nil
}
