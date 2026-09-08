package main

// 起動時CLIオプションのパース(仕様書§6.1)。
//
//	execsandbox-build -o <出力ファイル名> [--target <GOOS>/<GOARCH>] <入力.wasm>

import (
	"flag"
	"fmt"
	"io"
	"runtime"
)

// options はパース済みの起動時オプション一式。
type options struct {
	output    string
	target    target
	targetSet bool
	help      bool
	version   bool
	wasmPath  string // 位置引数(入力.wasm)
}

// parseArgs はargs(通常os.Args[1:])から起動時オプションを組み立てる。
// -h/--helpまたは-V/--versionが指定された場合、他の値の検証は行わずに
// optsを返す(呼び出し側でhelp/versionを先にチェックする)。
func parseArgs(args []string) (*options, error) {
	fs := flag.NewFlagSet("execsandbox-build", flag.ContinueOnError)
	// エラー・使い方の出力はflagパッケージに任せず、呼び出し側で
	// execsandbox-build:接頭辞付きの英語メッセージとして統一する
	// (.claude/rules/cli-output.md)。
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	opts := &options{}

	fs.StringVar(&opts.output, "o", "", "output file name (required)")
	fs.StringVar(&opts.output, "output", "", "output file name (required)")

	fs.Var(&targetValue{opts}, "target", "target platform, GOOS/GOARCH (default: this machine's platform)")

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

	if !opts.targetSet {
		t, ok := lookupTarget(runtime.GOOS, runtime.GOARCH)
		if !ok {
			return nil, fmt.Errorf("this machine's platform (%s/%s) is not one of the supported targets; specify --target explicitly (%s)", runtime.GOOS, runtime.GOARCH, supportedTargetsList())
		}
		opts.target = t
	}

	if err := opts.validate(fs.Args()); err != nil {
		return nil, err
	}

	return opts, nil
}

func (o *options) validate(positional []string) error {
	if o.output == "" {
		return fmt.Errorf("missing required -o/--output")
	}
	switch len(positional) {
	case 0:
		return fmt.Errorf("missing input .wasm file")
	case 1:
		o.wasmPath = positional[0]
		return nil
	default:
		return fmt.Errorf("expected exactly one input .wasm file, got %d (%v)", len(positional), positional)
	}
}

// --- --target: GOOS/GOARCH ---

type targetValue struct {
	opts *options
}

func (v *targetValue) String() string { return "" }

func (v *targetValue) Set(s string) error {
	t, err := parseTarget(s)
	if err != nil {
		return err
	}
	v.opts.target = t
	v.opts.targetSet = true
	return nil
}
