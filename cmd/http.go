package cmd

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/runutil"
	"github.com/spf13/cobra"
)

var (
	httpListen string
	httpStatus int
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Run an HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen, err := parseAddrPort(httpListen)
		if err != nil {
			return err
		}

		srv, err := httpserver.New(httpserver.Config{Addr: listen, StatusCode: httpStatus}, nil)
		if err != nil {
			return err
		}

		ctx, stop := runutil.SignalContext(context.Background())
		defer stop()

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.ListenAndServe()
		}()

		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
			return nil
		case err := <-errCh:
			if err == nil || errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			log.Printf("http: server error: %v", err)
			return err
		}
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)

	httpCmd.Flags().StringVar(&httpListen, "listen", ":8080", "listen address")
	httpCmd.Flags().IntVar(&httpStatus, "status", 200, "status code to return for all requests")
}
