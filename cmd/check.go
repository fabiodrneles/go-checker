package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/probe"
	"github.com/fabiodrneles/go-checker/internal/report"
	"github.com/fabiodrneles/go-checker/internal/runner"
	"github.com/spf13/cobra"
)

// defaultFiles are tried in order when neither -f nor URLs are given.
var defaultFiles = []string{"go-checker.yaml", "checks.yaml", "config.yaml"}

type checkFlags struct {
	file        string
	timeout     time.Duration
	retries     int
	concurrency int
	failOnWarn  bool
	noColor     bool
	trace       bool
	format      string
	output      string
	junit       string
}

func newCheck(env Env) *cobra.Command {
	var f checkFlags
	c := &cobra.Command{
		Use:   "check [URL...]",
		Short: "Run the checks once and report",
		Long: `Run every check once, concurrently, and print the verdict.

Checks come from the file given with -f, from URLs passed as arguments, or
from the first of go-checker.yaml, checks.yaml and config.yaml found in the
current directory.`,
		Example: `  go-checker check https://example.com
  go-checker check -f checks.yaml --timeout 3s --retries 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadChecks(f.file, args)
			if err != nil {
				return err
			}
			if err := validateFlags(f); err != nil {
				return err
			}
			override(cfg, cmd, f)
			start := time.Now()
			results := runner.Run(cmd.Context(), probe.New("go-checker/"+env.Version), cfg.Checks, runner.Options{Concurrency: f.concurrency})
			if err := writeReports(env, f, results, time.Since(start)); err != nil {
				return err
			}
			return verdict(results, f.failOnWarn)
		},
	}
	c.SetContext(context.Background())
	fl := c.Flags()
	fl.StringVarP(&f.file, "file", "f", "", "check file (default: go-checker.yaml, checks.yaml or config.yaml)")
	fl.DurationVar(&f.timeout, "timeout", config.DefaultTimeout, "timeout per attempt, overrides the file")
	fl.IntVar(&f.retries, "retries", 0, "retries after a network error or 5xx, overrides the file")
	fl.IntVarP(&f.concurrency, "concurrency", "c", 10, "checks running at the same time")
	fl.BoolVar(&f.failOnWarn, "fail-on-warn", false, "exit 1 on warnings too")
	fl.BoolVar(&f.noColor, "no-color", false, "disable colors (also NO_COLOR=1)")
	fl.BoolVar(&f.trace, "trace", false, "show DNS, connect, TLS and first-byte timings and the certificate")
	fl.StringVarP(&f.format, "format", "o", "text", "report format: text, json, junit or markdown")
	fl.StringVar(&f.output, "output", "", "write the report to this file (the terminal still shows text)")
	fl.StringVar(&f.junit, "junit", "", "also write a JUnit XML report to this file")
	return c
}

func loadChecks(file string, urls []string) (*config.Config, error) {
	if file != "" && len(urls) > 0 {
		return nil, usageError("use either -f or URLs, not both")
	}
	if len(urls) > 0 {
		cfg, err := config.FromURLs(urls)
		if err != nil {
			return nil, usageError("%v", err)
		}
		return cfg, nil
	}
	if file == "" {
		for _, name := range defaultFiles {
			if _, err := os.Stat(name); err == nil {
				file = name
				break
			}
		}
		if file == "" {
			return nil, usageError("no check file found (looked for go-checker.yaml, checks.yaml, config.yaml); pass -f FILE or URLs")
		}
	}
	cfg, err := config.Load(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, usageError("check file %q not found", file)
	}
	if err != nil {
		return nil, usageError("%v", err)
	}
	return cfg, nil
}

func validateFlags(f checkFlags) error {
	switch {
	case !formats[f.format]:
		return usageError("unknown --format %q (use text, json, junit or markdown)", f.format)
	case f.concurrency < 1:
		return usageError("--concurrency must be at least 1")
	case f.timeout <= 0:
		return usageError("--timeout must be positive")
	case f.retries < 0 || f.retries > config.MaxRetries:
		return usageError("--retries must be between 0 and %d", config.MaxRetries)
	}
	return nil
}

func override(cfg *config.Config, cmd *cobra.Command, f checkFlags) {
	for i := range cfg.Checks {
		if cmd.Flags().Changed("timeout") {
			cfg.Checks[i].Timeout.Duration = f.timeout
		}
		if cmd.Flags().Changed("retries") {
			r := f.retries
			cfg.Checks[i].Retries = &r
		}
	}
}

func verdict(rs []runner.Result, failOnWarn bool) error {
	s := runner.Summarize(rs)
	if s.Down > 0 || (failOnWarn && s.Warn > 0) {
		return errFailed
	}
	return nil
}

var formats = map[string]bool{"text": true, "json": true, "junit": true, "markdown": true}

// writeReports prints the report in the chosen format, to the terminal or to
// --output (then the terminal gets the text report), and appends the Markdown
// table to the GitHub Actions job summary when running there.
func writeReports(env Env, f checkFlags, rs []runner.Result, elapsed time.Duration) error {
	now := time.Now()
	doc := report.Build(rs, elapsed, "go-checker "+env.Version, now)
	color := !f.noColor && env.Getenv("NO_COLOR") == "" && env.IsTerminal != nil && env.IsTerminal()
	render := func(w io.Writer, format string) error {
		switch format {
		case "json":
			return report.JSON(w, doc)
		case "junit":
			return report.JUnit(w, doc, f.failOnWarn)
		case "markdown":
			return report.Markdown(w, doc)
		}
		return report.Text(w, rs, elapsed, report.TextOptions{Color: color && w == env.Stdout, Trace: f.trace, Now: now})
	}
	if f.output == "" {
		if err := render(env.Stdout, f.format); err != nil {
			return err
		}
	} else {
		if err := createFile(f.output, func(w io.Writer) error { return render(w, f.format) }); err != nil {
			return usageError("--output: %v", err)
		}
		if err := render(env.Stdout, "text"); err != nil {
			return err
		}
	}
	if f.junit != "" {
		if err := createFile(f.junit, func(w io.Writer) error { return report.JUnit(w, doc, f.failOnWarn) }); err != nil {
			return usageError("--junit: %v", err)
		}
	}
	if path := env.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		if err := appendFile(path, func(w io.Writer) error { return report.Markdown(w, doc) }); err != nil {
			_, _ = fmt.Fprintf(env.Stderr, "warning: job summary: %v\n", err)
		}
	}
	return nil
}

func createFile(path string, fn func(io.Writer) error) error {
	return withFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fn)
}

func appendFile(path string, fn func(io.Writer) error) error {
	return withFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, fn)
}

func withFile(path string, flag int, fn func(io.Writer) error) (err error) {
	fh, err := os.OpenFile(path, flag, 0o644) //nolint:gosec // the user picks the report path
	if err != nil {
		return err
	}
	defer func() {
		if cerr := fh.Close(); err == nil {
			err = cerr
		}
	}()
	return fn(fh)
}
