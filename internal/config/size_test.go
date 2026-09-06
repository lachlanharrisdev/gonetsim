package config

import "testing"

func TestParseSize(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"64MiB", 64 << 20, false},
		{"64mib", 64 << 20, false},
		{"512K", 512 << 10, false},
		{"1GiB", 1 << 30, false},
		{"4096", 4096, false},
		{"", 0, true},
		{"64GiB", 64 << 30, false},
		{"abc", 0, true},
		{"-1MiB", 0, true},
		{"64TiB", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseSize(tc.in)
		if tc.wantErr && err == nil {
			t.Errorf("ParseSize(%q): expected error", tc.in)
		}
		if !tc.wantErr && (err != nil || got != tc.want) {
			t.Errorf("ParseSize(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}
