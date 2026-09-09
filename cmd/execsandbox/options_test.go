package main

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "536870912", want: 536870912},
		{in: "512M", want: 512 * 1 << 20},
		{in: "512m", want: 512 * 1 << 20},
		{in: "512Mi", want: 512 * 1 << 20},
		{in: "512mi", want: 512 * 1 << 20},
		{in: "1K", want: 1 << 10},
		{in: "1G", want: 1 << 30},
		{in: "0", want: 0},
		{in: "", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "M", wantErr: true},
		{in: "-5M", wantErr: true},
		{in: "5X", wantErr: true},
		{in: "5MiB", wantErr: true}, // "Mi"の後にBは受理しない
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseSize(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseSize(%q) = %d, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSize(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseSize(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestEnvList_Set(t *testing.T) {
	var vars []envVar
	l := envList{&vars}

	if err := l.Set("KEY=VALUE"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := l.Set("EMPTY="); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := l.Set("WITH_EQUALS=a=b"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	want := []envVar{
		{Key: "KEY", Value: "VALUE"},
		{Key: "EMPTY", Value: ""},
		{Key: "WITH_EQUALS", Value: "a=b"},
	}
	if len(vars) != len(want) {
		t.Fatalf("vars = %#v, want %#v", vars, want)
	}
	for i := range want {
		if vars[i] != want[i] {
			t.Errorf("vars[%d] = %#v, want %#v", i, vars[i], want[i])
		}
	}

	for _, bad := range []string{"NOEQUALS", "=novalue"} {
		if err := l.Set(bad); err == nil {
			t.Errorf("Set(%q) = nil, want error", bad)
		}
	}
}

func TestParseVolume(t *testing.T) {
	tests := []struct {
		in      string
		want    volumeMount
		wantErr bool
	}{
		{in: "/data:/data", want: volumeMount{Host: "/data", Guest: "/data"}},
		{in: "/etc/conf:/conf:ro", want: volumeMount{Host: "/etc/conf", Guest: "/conf", ReadOnly: true}},
		{in: `C:\data:/data`, want: volumeMount{Host: `C:\data`, Guest: "/data"}},
		{in: `C:\data:/data:ro`, want: volumeMount{Host: `C:\data`, Guest: "/data", ReadOnly: true}},
		{in: "noseparator", wantErr: true},
		{in: "/data:", wantErr: true},
		{in: ":/data", wantErr: true},
		{in: "/data:relative", wantErr: true}, // ゲストパスは"/"始まりが必須
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseVolume(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseVolume(%q) = %#v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVolume(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseVolume(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestStdioSet(t *testing.T) {
	var s stdioSet
	if err := s.Set("in,out"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !s.In || !s.Out || s.Err {
		t.Errorf("s = %+v, want In=true Out=true Err=false", s)
	}

	var all stdioSet
	if err := all.Set("all"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !all.In || !all.Out || !all.Err {
		t.Errorf("all = %+v, want all true", all)
	}

	var bad stdioSet
	if err := bad.Set("bogus"); err == nil {
		t.Error("Set(bogus) = nil, want error")
	}
}

func TestAllowSet(t *testing.T) {
	var a allowSet
	if err := a.Set("random,time"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !a.Random || !a.Time {
		t.Errorf("a = %+v, want both true", a)
	}

	var viaAll allowSet
	if err := viaAll.Set("all"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !viaAll.Random || !viaAll.Time {
		t.Errorf("viaAll = %+v, want both true", viaAll)
	}

	var bad allowSet
	if err := bad.Set("bogus"); err == nil {
		t.Error("Set(bogus) = nil, want error")
	}
}

func TestDurationValue(t *testing.T) {
	var d durationValue
	if err := d.Set("30s"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if time.Duration(d) != 30*time.Second {
		t.Errorf("d = %v, want 30s", time.Duration(d))
	}

	for _, bad := range []string{"", "abc", "0s", "-5s"} {
		var d durationValue
		if err := d.Set(bad); err == nil {
			t.Errorf("Set(%q) = nil, want error", bad)
		}
	}
}

func TestDestAssignments_duplicateRejected(t *testing.T) {
	d := make(destAssignments)
	if err := d.Set("1=nodeB"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := d.Set("1=nodeC"); err == nil {
		t.Error("Set with a duplicate destination number = nil, want error")
	}
	if d[1] != "nodeB" {
		t.Errorf("d[1] = %q, want unchanged %q", d[1], "nodeB")
	}
}

func TestParseArgs_listen(t *testing.T) {
	opts, err := parseArgs([]string{"-l", "5432"})
	if err != nil {
		t.Fatalf("parseArgs(-l 5432): %v", err)
	}
	if opts.listen != "5432" {
		t.Errorf("listen = %q, want %q", opts.listen, "5432")
	}
}

func TestParseArgs_listen_invalidFormatIsRejected(t *testing.T) {
	if _, err := parseArgs([]string{"-l", "not-an-address"}); err == nil {
		t.Error("parseArgs(-l not-an-address) = nil, want error")
	}
}

func TestParseArgs_listen_repeatedIsRejected(t *testing.T) {
	if _, err := parseArgs([]string{"-l", "5432", "--listen", "5433"}); err == nil {
		t.Error("parseArgs with -l specified twice = nil, want error (only one listener is allowed, spec §4.1)")
	}
}

func TestParseArgs_helpAndVersionSkipValidation(t *testing.T) {
	opts, err := parseArgs([]string{"-h"})
	if err != nil {
		t.Fatalf("parseArgs(-h): %v", err)
	}
	if !opts.help {
		t.Error("opts.help = false, want true")
	}

	opts, err = parseArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("parseArgs(--version): %v", err)
	}
	if !opts.version {
		t.Error("opts.version = false, want true")
	}
}

func TestParseArgs_defaults(t *testing.T) {
	opts, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("parseArgs(nil): %v", err)
	}
	if opts.memLimit != defaultMemLimit {
		t.Errorf("memLimit = %d, want %d", opts.memLimit, defaultMemLimit)
	}
	if opts.mailboxLimit != defaultMailboxLimit {
		t.Errorf("mailboxLimit = %d, want %d", opts.mailboxLimit, defaultMailboxLimit)
	}
	if opts.maxFrame != defaultMaxFrame {
		t.Errorf("maxFrame = %d, want %d", opts.maxFrame, defaultMaxFrame)
	}
	if opts.timeout != 0 {
		t.Errorf("timeout = %v, want 0 (unset)", opts.timeout)
	}
}

func TestParseArgs_guestArgsAfterDoubleDash(t *testing.T) {
	opts, err := parseArgs([]string{"-n", "core", "--", "--verbose", "-a", "3"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if opts.name != "core" {
		t.Errorf("name = %q, want %q", opts.name, "core")
	}
	want := []string{"--verbose", "-a", "3"}
	if len(opts.guestArgs) != len(want) {
		t.Fatalf("guestArgs = %#v, want %#v", opts.guestArgs, want)
	}
	for i := range want {
		if opts.guestArgs[i] != want[i] {
			t.Errorf("guestArgs[%d] = %q, want %q", i, opts.guestArgs[i], want[i])
		}
	}
}

func TestParseArgs_invalidSizeIsRejected(t *testing.T) {
	if _, err := parseArgs([]string{"-m", "bogus"}); err == nil {
		t.Error("parseArgs(-m bogus) = nil, want error")
	}
}

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    options
		wantErr bool
	}{
		{name: "ok", opts: options{memLimit: 1, mailboxLimit: 1, maxFrame: 1}},
		{name: "zero memLimit", opts: options{memLimit: 0, mailboxLimit: 1, maxFrame: 1}, wantErr: true},
		{name: "zero mailboxLimit", opts: options{memLimit: 1, mailboxLimit: 0, maxFrame: 1}, wantErr: true},
		{name: "zero maxFrame", opts: options{memLimit: 1, mailboxLimit: 1, maxFrame: 0}, wantErr: true},
		{name: "maxFrame exceeds int32", opts: options{memLimit: 1, mailboxLimit: 1, maxFrame: math.MaxInt32 + 1}, wantErr: true},
		{name: "maxFrame at int32 limit", opts: options{memLimit: 1, mailboxLimit: 1, maxFrame: math.MaxInt32}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.validate()
			if tt.wantErr != (err != nil) {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOptionsValidate_volumeHostMustExist(t *testing.T) {
	base := options{memLimit: 1, mailboxLimit: 1, maxFrame: 1}

	ok := base
	ok.volumes = []volumeMount{{Host: t.TempDir(), Guest: "/data"}}
	if err := ok.validate(); err != nil {
		t.Errorf("validate() with an existing host path = %v, want nil", err)
	}

	bad := base
	bad.volumes = []volumeMount{{Host: filepath.Join(t.TempDir(), "does-not-exist"), Guest: "/data"}}
	if err := bad.validate(); err == nil {
		t.Error("validate() with a nonexistent host path = nil, want error")
	}
}
