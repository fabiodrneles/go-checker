// Command go-checker verifies that websites and APIs are up and behave as expected.
package main

import (
	"os"

	"github.com/fabiodrneles/go-checker/cmd"
)

// Set by GoReleaser through -ldflags.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:], cmd.Env{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
		Commit:  commit,
		Getenv:  os.Getenv,
		IsTerminal: func() bool {
			fi, err := os.Stdout.Stat()
			return err == nil && fi.Mode()&os.ModeCharDevice != 0
		},
	}))
}
