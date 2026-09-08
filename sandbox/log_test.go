package sandbox

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger_Printf_prefixAndFormat(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, false)

	l.Printf("dropped %d message(s)", 3)

	got := buf.String()
	if !strings.HasPrefix(got, "execsandbox: ") {
		t.Errorf("log = %q, want execsandbox: prefix", got)
	}
	if !strings.Contains(got, "dropped 3 message(s)") {
		t.Errorf("log = %q, want it to contain the formatted message", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("log = %q, want a trailing newline", got)
	}
}

func TestLogger_Printf_quietSuppressesOutput(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, true)

	l.Printf("this must not appear")

	if buf.Len() != 0 {
		t.Errorf("log = %q, want empty output when quiet", buf.String())
	}
}
