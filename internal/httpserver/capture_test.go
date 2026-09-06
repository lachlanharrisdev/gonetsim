// //----------------------------------------------------------------------------
// // NOTICE: to save development time, test files (including this) have been
// // generated with LLMs. The author(s) do not claim credit for these tests
// // and exist purely for maximising code quality and reliability
// //
// // For more information please see `/.github/AI_USAGE.md`
// //----------------------------------------------------------------------------//

package httpserver

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
)

func TestService_CapturesHTTP(t *testing.T) {
	conf := Config{
		Addr:       testutil.FreeTCPAddr(t),
		StatusCode: http.StatusOK,
		Mode:       "fake",
		Capture:    true,
	}
	run, path := testutil.NewPcapRun(t)
	svc, errCh := startHTTPService(t, conf, run)

	get := func(url string) *http.Response {
		_, resp := testutil.RetryGet(t, http.DefaultClient, url)
		return resp
	}
	get("http://" + conf.Addr + "/warmup")
	resp := get("http://" + conf.Addr + "/hello")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	testutil.WaitForPayloadContains(t, path, "GET /hello", 3*time.Second)
	testutil.WaitForPayloadContains(t, path, "HTTP/1.1 200", 3*time.Second)

	_ = svc.Stop(context.Background())
	testutil.DiscardServiceStartErr(t, errCh)
}

func startHTTPService(t *testing.T, conf Config, run *capture.Run) (service.Service, <-chan error) {
	t.Helper()
	logger := testutil.Logger()
	svc := NewService(conf, logger, run)

	errCh := make(chan error, 1)
	go func() { errCh <- svc.Start(context.Background()) }()
	return svc, errCh
}
