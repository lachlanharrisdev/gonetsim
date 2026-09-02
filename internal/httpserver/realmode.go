package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type RealHandler struct {
	// same statuscode implementation as fake mode
	StatusCode int
	RootDir    string
	Logger     *slog.Logger
}

func (h RealHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.Logger

	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	target := filepath.Join(h.RootDir, filepath.FromSlash(reqPath))

	absRoot, err := filepath.Abs(h.RootDir)
	if err != nil {
		logger.Error("could not resolve root directory", "root", h.RootDir, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		logger.Error("could not resolve requested path", "path", r.URL.Path, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !pathWithin(absRoot, absTarget) {
		logger.Warn("directory traversal blocked", "src", r.RemoteAddr, "path", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("file not found", "path", r.URL.Path)
			http.NotFound(w, r)
		} else {
			logger.Error("error opening file", "path", r.URL.Path, "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		logger.Error("error statting file", "path", r.URL.Path, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Directories are served via their index.html when one exists.
	if stat.IsDir() {
		_ = f.Close()
		index := filepath.Join(absTarget, "index.html")
		f, err = os.Open(index)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Debug("directory listing blocked", "path", r.URL.Path)
			} else {
				logger.Error("error opening directory index", "path", r.URL.Path, "err", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			http.NotFound(w, r)
			return
		}
		stat, err = f.Stat()
		if err != nil {
			_ = f.Close()
			logger.Error("error statting directory index", "path", r.URL.Path, "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if stat.IsDir() {
			_ = f.Close()
			logger.Debug("directory index is a directory", "path", r.URL.Path)
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()

	cap := &statusCaptureWriter{ResponseWriter: w}
	out := http.ResponseWriter(cap)
	if h.StatusCode != 0 {
		out = &statusOverrideWriter{ResponseWriter: cap, status: h.StatusCode}
	}

	http.ServeContent(out, r, stat.Name(), stat.ModTime(), f)

	status := cap.status
	if status == 0 {
		status = http.StatusOK
	}
	logger.Info(
		r.Method,
		"src", r.RemoteAddr,
		"to", r.URL.Path,
		"status", status,
		"host", r.Host,
		"ua", r.UserAgent(),
		"len", fmt.Sprintf("%d", stat.Size()),
	)
}

// pathWithin reports whether child is inside parent (or equals it), using only
// lexical path components. Both arguments must already be absolute.
func pathWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return !filepath.IsAbs(rel)
}
