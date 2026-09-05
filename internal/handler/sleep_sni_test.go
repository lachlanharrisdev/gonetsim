package handler

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestLuaSleepAndSNI(t *testing.T) {
	t.Run("sleep resets idle deadline", func(t *testing.T) {
		h, err := NewLua("testdata/sleep.lua", nil)
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: discardLogger(), IdleTimeout: 100 * time.Millisecond})

		// the read after sleep(200) must still succeed even though more
		// than IdleTimeout passed since the first read
		go func() {
			_, _ = client.Write([]byte("go"))
			time.Sleep(50 * time.Millisecond)
			_, _ = client.Write([]byte("again"))
		}()

		start := time.Now()
		buf := make([]byte, len("after-sleep"))
		if _, err := io.ReadFull(client, buf); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if string(buf) != "after-sleep" {
			t.Fatalf("unexpected reply %q", buf)
		}
		if time.Since(start) < 150*time.Millisecond {
			t.Fatalf("sleep(200) returned early")
		}
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
	})

	t.Run("sleep cap", func(t *testing.T) {
		h, err := NewLua("testdata/sleep_cap.lua", nil)
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: discardLogger()})
		_, _ = client.Write([]byte("go"))
		if err := <-done; err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("expected sleep cap error, got: %v", err)
		}
	})

	t.Run("sni nil on plain conn", func(t *testing.T) {
		h, err := NewLua("testdata/sni.lua", nil)
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: discardLogger()})
		buf := make([]byte, 6)
		if _, err := io.ReadFull(client, buf); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if string(buf) != "no-sni" {
			t.Fatalf("expected no-sni, got %q", buf)
		}
		_ = client.Close()
		<-done
	})
}
