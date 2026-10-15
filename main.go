// Command go-checker verifies that websites and APIs are up and behave as expected.
package main

import (
	"os"
	"regexp"
	"runtime/debug"

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
		Version: resolveVersion(version, debug.ReadBuildInfo),
		Commit:  commit,
		Getenv:  os.Getenv,
		IsTerminal: func() bool {
			fi, err := os.Stdout.Stat()
			return err == nil && fi.Mode()&os.ModeCharDevice != 0
		},
	}))
}

// releaseVersion matches tagged versions; pseudoVersion matches what Go stamps
// on untagged builds (v0.0.0-20260101000000-abcdef123456), which stay "dev".
var (
	releaseVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$`)
	pseudoVersion  = regexp.MustCompile(`\d{14}-[0-9a-f]{12}`)
)

// resolveVersion falls back to the module version recorded by
// "go install github.com/fabiodrneles/go-checker@vX.Y.Z" when the build did
// not embed one through -ldflags (spec 001 FR-5).
func resolveVersion(v string, info func() (*debug.BuildInfo, bool)) string {
	if v != "dev" {
		return v
	}
	if bi, ok := info(); ok && releaseVersion.MatchString(bi.Main.Version) && !pseudoVersion.MatchString(bi.Main.Version) {
		return bi.Main.Version
	}
	return v
}
