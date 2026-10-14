// Package cmd wires the command-line interface.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Exit codes, documented in the README: scripts and CI rely on them.
const (
	ExitOK    = 0 // every check passed
	ExitFail  = 1 // at least one check is down (or warn, with --fail-on-warn)
	ExitUsage = 2 // bad flags, unreadable or invalid check file
)

// Env is everything the commands touch outside themselves, so that tests can
// run them in-process.
type Env struct {
	Stdout, Stderr io.Writer
	Version        string
	Commit         string
	Getenv         func(string) string
	// IsTerminal reports whether Stdout is a terminal (colors); nil means no.
	IsTerminal func() bool
}

// exitError carries an exit code up to Execute.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }

func usageError(format string, a ...any) error {
	return &exitError{code: ExitUsage, err: fmt.Errorf(format, a...)}
}

// errFailed signals failing checks; the report already said why.
var errFailed = &exitError{code: ExitFail, err: errors.New("checks failed")}

// Execute runs the CLI and returns the process exit code.
func Execute(args []string, env Env) int {
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}
	root := newRoot(env)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return ExitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee != errFailed {
			_, _ = fmt.Fprintf(env.Stderr, "error: %v\n", ee.err)
		}
		return ee.code
	}
	// Anything cobra rejects (unknown flag or command) is a usage error.
	_, _ = fmt.Fprintf(env.Stderr, "error: %v\nRun 'go-checker --help' for usage.\n", err)
	return ExitUsage
}

func newRoot(env Env) *cobra.Command {
	root := &cobra.Command{
		Use:   "go-checker",
		Short: "Health checks for websites and APIs, from your terminal or your CI",
		Long: `go-checker verifies that websites and APIs are up and behave as expected.

Describe your endpoints in a YAML file (or pass URLs directly), run
"go-checker check" and read the verdict, or let the exit code drive your
pipeline: 0 all up, 1 something is down, 2 usage or configuration error.`,
		Version:       fmt.Sprintf("%s (%s)", env.Version, env.Commit),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	root.SetVersionTemplate("go-checker {{.Version}}\n")
	root.AddCommand(newCheck(env), newVersion(env))
	return root
}

func newVersion(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(env.Stdout, "go-checker %s (%s)\n", env.Version, env.Commit)
			return err
		},
	}
}
