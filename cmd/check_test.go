package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
)

func TestCheckRunDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	if err := checkRunDir(); err != nil {
		t.Fatalf("checkRunDir: %v", err)
	}
	runs, err := capture.DefaultRunsDir()
	if err != nil {
		t.Fatalf("DefaultRunsDir: %v", err)
	}
	if st, err := os.Stat(runs); err != nil || !st.IsDir() {
		t.Fatalf("expected runs dir to exist: %v", err)
	}
	if filepath.Dir(runs) != filepath.Join(dir, "gonetsim") {
		t.Fatalf("runs dir = %q, want it under %q", runs, dir)
	}
}
