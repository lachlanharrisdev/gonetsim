package state

import (
	"strings"
	"testing"
)

func TestState(t *testing.T) {
	t.Run("roundtrip and accounting", func(t *testing.T) {
		b := NewBudget(1024)
		s := NewStore(b)

		if err := s.Set("k", "\x00\xffbinary\x00"); err != nil {
			t.Fatalf("Set binary: %v", err)
		}
		v, ok := s.Get("k")
		if !ok || v != "\x00\xffbinary\x00" {
			t.Fatalf("binary roundtrip = %q, %v", v, ok)
		}
		if !s.Has("k") {
			t.Fatalf("expected key to exist")
		}

		// replacing a value must not double-count it
		if err := s.Set("k", strings.Repeat("A", 512)); err != nil {
			t.Fatalf("Set replace: %v", err)
		}
		if b.used != 512 {
			t.Fatalf("usage after replace = %d, want 512", b.used)
		}
		if _, ok := s.Get("missing"); ok {
			t.Fatalf("expected missing key to be absent")
		}

		s.Delete("k")
		if s.Has("k") || b.used != 0 {
			t.Fatalf("delete: still present, usage %d", b.used)
		}
		// deleting a missing key is a no-op
		s.Delete("k")
	})

	t.Run("shared budget", func(t *testing.T) {
		b := NewBudget(10)
		a, c := NewStore(b), NewStore(b)

		if err := a.Set("x", strings.Repeat("A", 7)); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := c.Set("y", strings.Repeat("B", 3)); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := c.Set("z", "overflow"); err == nil || !strings.Contains(err.Error(), "state limit") {
			t.Fatalf("expected limit error, got: %v", err)
		}

		// freeing in one store makes room in the other
		a.Delete("x")
		if err := c.Set("z", strings.Repeat("C", 7)); err != nil {
			t.Fatalf("Set after free: %v", err)
		}
	})

	t.Run("caps", func(t *testing.T) {
		s := NewStore(nil)
		if err := s.Set("", "v"); err == nil {
			t.Fatalf("expected empty key error")
		}
		if err := s.Set(strings.Repeat("k", MaxKeyLen+1), "v"); err == nil {
			t.Fatalf("expected key cap error")
		}
		if err := s.Set("k", strings.Repeat("v", MaxValueLen+1)); err == nil {
			t.Fatalf("expected value cap error")
		}
		if err := s.Set("k", ""); err != nil {
			t.Fatalf("empty value should be allowed: %v", err)
		}
	})

	t.Run("parse size", func(t *testing.T) {
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
	})
}
