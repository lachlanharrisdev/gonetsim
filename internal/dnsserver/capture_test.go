// //----------------------------------------------------------------------------
// // NOTICE: to save development time, test files (including this) have been
// // generated with LLMs. The author(s) do not claim credit for these tests
// // and exist purely for maximising code quality and reliability
// //
// // For more information please see `/.github/AI_USAGE.md`
// //----------------------------------------------------------------------------//

package dnsserver

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
)

func TestService_Captures(t *testing.T) {
	for _, network := range []string{"udp", "tcp"} {
		t.Run(network, func(t *testing.T) {
			conf := baseCaptureConfig(t, network)
			conf.Capture = true
			run, path := testutil.NewPcapRun(t)

			svc, errCh := startDNSService(t, conf, run)

			query := newAQuery()
			client := &dns.Client{Net: network, Timeout: 1 * time.Second}
			if _, _, err := testutil.RetryDNSExchange(t, client, conf.Addr, query); err != nil {
				t.Fatalf("exchange: %v", err)
			}

			testutil.WaitForPayloadContains(t, path, "example", 3*time.Second)
			testutil.WaitForPayload(t, path, 3*time.Second, func(s string) bool {
				return strings.Count(s, "example") >= 2 // query + response
			})

			_ = svc.Stop(context.Background())
			testutil.DiscardServiceStartErr(t, errCh)
		})
	}
}

func baseCaptureConfig(t *testing.T, network string) Config {
	t.Helper()
	return Config{
		Addr:           testutil.FreePort(t, "tcp"),
		Net:            network,
		SinkholeIPv4:   netip.MustParseAddr("203.0.113.10"),
		SinkholeIPv6:   netip.MustParseAddr("2001:db8::10"),
		SinkholeDomain: "localhost",
		SinkholeTXT:    "test",
		TTL:            60,
		Compress:       false,
	}
}

func newAQuery() *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	return m
}

func startDNSService(t *testing.T, conf Config, run *capture.Run) (service.Service, <-chan error) {
	t.Helper()
	logger := testutil.Logger()
	svc := NewService(conf, logger, run)

	errCh := make(chan error, 1)
	go func() { errCh <- svc.Start(context.Background()) }()
	return svc, errCh
}
