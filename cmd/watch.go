package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fabiodrneles/go-checker/internal/metrics"
	"github.com/fabiodrneles/go-checker/internal/notify"
	"github.com/fabiodrneles/go-checker/internal/probe"
	"github.com/fabiodrneles/go-checker/internal/runner"
	"github.com/fabiodrneles/go-checker/internal/watch"
	"github.com/spf13/cobra"
)

type watchFlags struct {
	checkFlags
	interval    time.Duration
	metricsAddr string
	confirm     int
	window      int
	webhooks    []string
	slack       []string
}

func newWatch(env Env) *cobra.Command {
	var f watchFlags
	c := &cobra.Command{
		Use:   "watch [URL...]",
		Short: "Run the checks continuously, expose Prometheus metrics and alert on changes",
		Long: `Run the checks every --interval until interrupted (Ctrl+C or SIGTERM).

With --metrics-addr, /metrics serves Prometheus metrics (up, latency p50/p95/p99
over the last --window responses, checks by state, certificate days left)
and /healthz answers 200. A state change is reported, and sent to every
--webhook and --slack URL, only after it lasts --confirm rounds in a row,
so that one lost packet does not page anyone.`,
		Example: `  go-checker watch -f checks.yaml --interval 30s --metrics-addr :9090
  go-checker watch https://example.com --confirm 3 --slack https://hooks.slack.com/services/...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadChecks(f.file, args)
			if err != nil {
				return err
			}
			if err := validateFlags(f.checkFlags); err != nil {
				return err
			}
			switch {
			case f.interval < time.Second:
				return usageError("--interval must be at least 1s")
			case f.confirm < 1:
				return usageError("--confirm must be at least 1")
			case f.window < 1:
				return usageError("--window must be at least 1")
			}
			override(cfg, cmd, f.checkFlags)
			var ns []notify.Notifier
			for _, u := range f.webhooks {
				ns = append(ns, notify.Webhook{URL: u})
			}
			for _, u := range f.slack {
				ns = append(ns, notify.Slack{URL: u})
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			reg := metrics.New(f.window)
			if f.metricsAddr != "" {
				ln, err := net.Listen("tcp", f.metricsAddr)
				if err != nil {
					return usageError("--metrics-addr: %v", err)
				}
				srv := &http.Server{Handler: MetricsHandler(reg), ReadHeaderTimeout: 5 * time.Second}
				go func() { _ = srv.Serve(ln) }()
				defer func() {
					sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					_ = srv.Shutdown(sctx)
				}()
				_, _ = fmt.Fprintf(env.Stdout, "metrics on http://%s/metrics\n", ln.Addr())
			}
			_, _ = fmt.Fprintf(env.Stdout, "watching %s every %s (alerts after %d rounds)\n", plural(len(cfg.Checks), "check"), f.interval, f.confirm)
			w := &watch.Watcher{
				Checks:    cfg.Checks,
				Doer:      probe.New("go-checker/" + env.Version),
				Options:   runner.Options{Concurrency: f.concurrency},
				Interval:  f.interval,
				Confirm:   f.confirm,
				Metrics:   reg,
				Notifiers: ns,
				Out:       env.Stdout,
				Errs:      env.Stderr,
			}
			if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			_, _ = fmt.Fprintln(env.Stdout, "stopped")
			return nil
		},
	}
	fl := c.Flags()
	fl.StringVarP(&f.file, "file", "f", "", "check file (default: go-checker.yaml, checks.yaml or config.yaml)")
	fl.DurationVar(&f.timeout, "timeout", 10*time.Second, "timeout per attempt, overrides the file")
	fl.IntVar(&f.retries, "retries", 0, "retries after a network error or 5xx, overrides the file")
	fl.IntVarP(&f.concurrency, "concurrency", "c", 10, "checks running at the same time")
	fl.DurationVar(&f.interval, "interval", 30*time.Second, "time between rounds")
	fl.StringVar(&f.metricsAddr, "metrics-addr", "", `serve Prometheus metrics on this address (e.g. ":9090")`)
	fl.IntVar(&f.confirm, "confirm", 2, "rounds a new state must last before it is reported")
	fl.IntVar(&f.window, "window", 100, "latest responses used for the latency quantiles")
	fl.StringArrayVar(&f.webhooks, "webhook", nil, "POST each state change as JSON to this URL (repeatable)")
	fl.StringArrayVar(&f.slack, "slack", nil, "send each state change to this Slack incoming webhook (repeatable)")
	f.format = "text"
	return c
}

// MetricsHandler serves /metrics and /healthz.
func MetricsHandler(reg *metrics.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_ = reg.Write(w)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	return mux
}
