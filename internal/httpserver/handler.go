package httpserver

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type Handler struct {
	StatusCode int
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := h.StatusCode
	if status == 0 {
		status = http.StatusOK
	}

	bodyPreview := ""
	if r.Body != nil {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 1024))
		_ = r.Body.Close()
		bodyPreview = strings.TrimSpace(string(b))
	}

	log.Printf(
		"http: %s %s host=%q ua=%q from=%s len=%d body=%q",
		r.Method,
		r.URL.String(),
		r.Host,
		r.UserAgent(),
		r.RemoteAddr,
		r.ContentLength,
		bodyPreview,
	)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "gonetsim\nmethod=%s\npath=%s\n", r.Method, r.URL.Path)
}
