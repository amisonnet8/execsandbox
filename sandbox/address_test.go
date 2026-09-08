package sandbox

import "testing"

func TestResolveSocketPath(t *testing.T) {
	tests := []struct {
		name          string
		goos          string
		xdgRuntimeDir string
		localAppData  string
		uid           int
		id            string
		want          string
		wantErr       bool
	}{
		{
			name:          "unix with XDG_RUNTIME_DIR",
			goos:          "linux",
			xdgRuntimeDir: "/run/user/1000",
			id:            "dbcore",
			want:          "/run/user/1000/execsandbox/dbcore.sock",
		},
		{
			name: "unix without XDG_RUNTIME_DIR falls back to /tmp",
			goos: "linux",
			uid:  1000,
			id:   "dbcore",
			want: "/tmp/execsandbox-1000/dbcore.sock",
		},
		{
			name:         "windows uses LOCALAPPDATA",
			goos:         "windows",
			localAppData: `C:\Users\alice\AppData\Local`,
			id:           "dbcore",
			want:         `C:\Users\alice\AppData\Local\execsandbox\dbcore.sock`,
		},
		{
			name:    "windows without LOCALAPPDATA is an error",
			goos:    "windows",
			id:      "dbcore",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSocketPath(tt.goos, tt.xdgRuntimeDir, tt.localAppData, tt.uid, tt.id)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveSocketPath() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSocketPath() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveSocketPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
