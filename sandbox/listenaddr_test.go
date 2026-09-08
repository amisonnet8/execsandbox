package sandbox

import "testing"

func TestParseListenAddress(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantNetwork string
		wantAddress string
		wantErr     bool
	}{
		{name: "port only", in: "5432", wantNetwork: "tcp", wantAddress: "127.0.0.1:5432"},
		{name: "ipv4 with port", in: "127.0.0.1:5432", wantNetwork: "tcp", wantAddress: "127.0.0.1:5432"},
		{name: "specific interface", in: "192.168.1.10:5432", wantNetwork: "tcp", wantAddress: "192.168.1.10:5432"},
		{name: "all interfaces", in: ":5432", wantNetwork: "tcp", wantAddress: ":5432"},
		{name: "ipv6 loopback", in: "[::1]:5432", wantNetwork: "tcp", wantAddress: "[::1]:5432"},
		{name: "unix socket path", in: "/run/mydb.sock", wantNetwork: "unix", wantAddress: "/run/mydb.sock"},
		{name: "unix: prefix", in: "unix:/run/mydb.sock", wantNetwork: "unix", wantAddress: "/run/mydb.sock"},
		{
			name:        "unix: prefix with windows drive letter path",
			in:          `unix:C:\run\mydb.sock`,
			wantNetwork: "unix",
			wantAddress: `C:\run\mydb.sock`,
		},
		{name: "empty", in: "", wantErr: true},
		{name: "unix: with empty path", in: "unix:", wantErr: true},
		{name: "port zero", in: "0", wantErr: true},
		{name: "port too large", in: "65536", wantErr: true},
		{name: "non-numeric port", in: "127.0.0.1:bogus", wantErr: true},
		{name: "garbage", in: "not-an-address", wantErr: true},
		{
			// Windowsのドライブレターは"unix:"接頭辞なしでは受理しない
			// （先頭が"/"でもポート番号形式でもないため、HOST:PORTとして
			// 解釈を試みて失敗する）。
			name:    "bare windows drive letter path is rejected",
			in:      `C:\run\mydb.sock`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, address, err := ParseListenAddress(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseListenAddress(%q) = (%q, %q), want an error", tt.in, network, address)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseListenAddress(%q) error = %v", tt.in, err)
			}
			if network != tt.wantNetwork || address != tt.wantAddress {
				t.Errorf("ParseListenAddress(%q) = (%q, %q), want (%q, %q)", tt.in, network, address, tt.wantNetwork, tt.wantAddress)
			}
		})
	}
}
