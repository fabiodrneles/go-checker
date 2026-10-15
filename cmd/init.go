package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/openapi"
	"github.com/spf13/cobra"
)

// starter is the file "init" writes without --from-openapi.
const starter = `# go-checker checks. Run them with: go-checker check
# Reference: https://github.com/fabiodrneles/go-checker#configuration

defaults:
  timeout: 5s        # per attempt
  retries: 1         # after a network error or a 5xx, with backoff
  tls_min_days: 14   # warn when the certificate expires sooner

checks:
  - name: Homepage
    url: https://example.com
    expect:
      status: 200
      max_latency: 1s          # slower is a warning
      body_contains: Example

  # - name: API health
  #   url: https://api.example.com/health
  #   headers: {Authorization: Bearer TOKEN}
  #   expect:
  #     status: [200, 204]
  #     headers: {Content-Type: application/json}
  #     json:
  #       - {path: $.status, equals: ok}
`

func newInit(env Env) *cobra.Command {
	var from, base, out string
	var force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Write a starter check file, or generate one from an OpenAPI spec",
		Long: `Write a commented starter go-checker.yaml, or generate one check per GET
operation of an OpenAPI 3 spec (YAML or JSON). Operations that need a path
parameter or a required query, header or cookie parameter are listed as
skipped; the expected status is the lowest documented 2xx.`,
		Example: `  go-checker init
  go-checker init --from-openapi openapi.yaml --base-url http://localhost:8080
  go-checker init --from-openapi openapi.json -f - | less`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			data := []byte(starter)
			skipped := 0
			if from != "" {
				src, err := os.ReadFile(from)
				if err != nil {
					return usageError("%v", err)
				}
				doc, err := openapi.Parse(src)
				if err != nil {
					return usageError("%s: %v", from, err)
				}
				res, err := openapi.Generate(doc, base)
				if err != nil {
					return usageError("%s: %v", from, err)
				}
				if data, err = res.YAML(); err != nil {
					return err
				}
				skipped = len(res.Skipped)
			} else if base != "" {
				return usageError("--base-url needs --from-openapi")
			}
			if _, err := config.Parse(data); err != nil {
				return fmt.Errorf("generated file is invalid (please report it): %w", err)
			}
			if out == "-" {
				_, err := env.Stdout.Write(data)
				return err
			}
			if _, err := os.Stat(out); err == nil && !force {
				return usageError("%s already exists (use --force to overwrite)", out)
			} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return usageError("%v", err)
			}
			if err := os.WriteFile(out, data, 0o644); err != nil { //nolint:gosec // a config file, meant to be readable
				return usageError("%v", err)
			}
			cfg, _ := config.Parse(data)
			msg := fmt.Sprintf("wrote %s with %s", out, plural(len(cfg.Checks), "check"))
			if skipped > 0 {
				msg += fmt.Sprintf(" (%d operations skipped, see the header)", skipped)
			}
			_, err := fmt.Fprintf(env.Stdout, "%s\nnext: go-checker check -f %s\n", msg, out)
			return err
		},
	}
	fl := c.Flags()
	fl.StringVar(&from, "from-openapi", "", "OpenAPI 3 spec (YAML or JSON) to generate the checks from")
	fl.StringVar(&base, "base-url", "", "base URL of the API (default: the spec's first server)")
	fl.StringVarP(&out, "file", "f", "go-checker.yaml", `file to write ("-" for standard output)`)
	fl.BoolVar(&force, "force", false, "overwrite an existing file")
	return c
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
