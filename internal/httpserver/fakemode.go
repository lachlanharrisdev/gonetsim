package httpserver

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed content/index.html
var defaultIndexHTML []byte

//go:embed content/index.txt
var defaultIndexTXT []byte

//go:embed content/index.json
var defaultIndexJSON []byte

//go:embed content/index.xml
var defaultIndexXML []byte

//go:embed content/index.css
var defaultIndexCSS []byte

//go:embed content/index.js
var defaultIndexJS []byte

type FakeHandler struct {
	// if non-zero, forces this status code for all responses.
	StatusCode int
	Logger     *slog.Logger
}

type fakeMeta struct {
	cleanPath string
	name      string
	ext       string
}

type fakeResponse struct {
	contentType string
	modTime     time.Time
	body        []byte
}

type fakeGenerator func(r *http.Request, m fakeMeta) fakeResponse

type statusOverrideWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusOverrideWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if w.status != 0 {
		w.ResponseWriter.WriteHeader(w.status)
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

type statusCaptureWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (h FakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.Logger

	m := resolveFakeMeta(r.URL.Path)
	gen := defaultFakeRegistry.lookup(m.ext)
	resp := gen(r, m)

	if resp.contentType != "" {
		w.Header().Set("Content-Type", resp.contentType)
	}

	cap := &statusCaptureWriter{ResponseWriter: w}
	out := http.ResponseWriter(cap)
	if h.StatusCode != 0 {
		out = &statusOverrideWriter{ResponseWriter: cap, status: h.StatusCode}
	}

	http.ServeContent(out, r, m.name, resp.modTime, bytes.NewReader(resp.body))

	status := cap.status
	if status == 0 {
		// ServeContent defaults to 200 if it wrote a body.
		status = 200
	}
	logger.Info(
		r.Method,
		"src", r.RemoteAddr,
		"to", r.URL.Path,
		"status", status,
		"host", r.Host,
		"ua", r.UserAgent(),
		"len", r.ContentLength,
	)
}

func resolveFakeMeta(urlPath string) fakeMeta {
	p := urlPath
	if p == "" {
		p = "/"
	}
	isDir := strings.HasSuffix(p, "/")
	clean := path.Clean("/" + p)
	name := path.Base(clean)
	ext := strings.ToLower(path.Ext(name))
	if isDir || ext == "" {
		ext = ".html"
		name = "index.html"
	}
	return fakeMeta{cleanPath: clean, name: name, ext: ext}
}

type fakeRegistry struct {
	byExt      map[string]fakeGenerator
	defaultGen fakeGenerator
}

func (r *fakeRegistry) lookup(ext string) fakeGenerator {
	if r == nil {
		return defaultFakeRegistry.defaultGen
	}
	if g, ok := r.byExt[ext]; ok {
		return g
	}
	return r.defaultGen
}

var defaultFakeRegistry = newDefaultFakeRegistry()

func newDefaultFakeRegistry() *fakeRegistry {
	mod := time.Unix(0, 0).UTC()

	htmlGen := func(_ *http.Request, _ fakeMeta) fakeResponse {
		return fakeResponse{contentType: "text/html; charset=utf-8", modTime: mod, body: defaultIndexHTML}
	}

	txtGen := func(_ *http.Request, m fakeMeta) fakeResponse {
		return fakeResponse{contentType: "text/plain; charset=utf-8", modTime: mod, body: defaultIndexTXT}
	}

	jsonGen := func(_ *http.Request, m fakeMeta) fakeResponse {
		return fakeResponse{contentType: "application/json; charset=utf-8", modTime: mod, body: defaultIndexJSON}
	}

	xmlGen := func(_ *http.Request, m fakeMeta) fakeResponse {
		return fakeResponse{contentType: "application/xml; charset=utf-8", modTime: mod, body: defaultIndexXML}
	}

	cssGen := func(_ *http.Request, m fakeMeta) fakeResponse {
		return fakeResponse{contentType: "text/css; charset=utf-8", modTime: mod, body: defaultIndexCSS}
	}

	jsGen := func(_ *http.Request, m fakeMeta) fakeResponse {
		return fakeResponse{contentType: "application/javascript; charset=utf-8", modTime: mod, body: defaultIndexJS}
	}

	unknownGen := func(_ *http.Request, m fakeMeta) fakeResponse {
		sum := sha256.Sum256([]byte(m.cleanPath + "|" + m.ext))
		body := make([]byte, 512)
		for i := 0; i < len(body); i++ {
			body[i] = sum[i%len(sum)]
		}

		ct := mime.TypeByExtension(m.ext)
		if ct == "" {
			ct = "application/octet-stream"
		}
		return fakeResponse{contentType: ct, modTime: mod, body: body}
	}

	byExt := map[string]fakeGenerator{
		// default / directory
		".html": htmlGen,
		".htm":  htmlGen,

		// common dynamic-page extensions (fake)
		".php":  htmlGen,
		".asp":  htmlGen,
		".aspx": htmlGen,
		".jsp":  htmlGen,

		// text-ish
		".txt": txtGen,
		".log": txtGen,
		".md":  txtGen,

		// structured
		".json": jsonGen,
		".xml":  xmlGen,

		// web
		".css": cssGen,
		".js":  jsGen,
	}

	return &fakeRegistry{byExt: byExt, defaultGen: unknownGen}
}
