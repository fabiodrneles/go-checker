// Package config loads and validates the go-checker check file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Defaults applied when neither the check nor the file sets a value.
const (
	DefaultTimeout = 10 * time.Second
	DefaultMethod  = "GET"
	MaxRetries     = 10
	// DefaultTLSMinDays is how close to expiry a certificate may get before
	// the check turns WARN (spec 003 FR-3).
	DefaultTLSMinDays = 14
)

// Duration is a time.Duration written in YAML as "500ms", "5s" or "1m".
type Duration struct{ time.Duration }

// UnmarshalYAML parses a Go duration string.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return fmt.Errorf("line %d: duration must be a string such as \"5s\"", n.Line)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: invalid duration %q (use 500ms, 5s, 1m)", n.Line, s)
	}
	d.Duration = v
	return nil
}

// MarshalYAML writes the duration back as a string.
func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

// Defaults holds file-wide values that every check inherits.
type Defaults struct {
	Timeout    Duration          `yaml:"timeout,omitempty"`
	Retries    *int              `yaml:"retries,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	TLSMinDays *int              `yaml:"tls_min_days,omitempty"`
}

// Check is one endpoint to verify.
type Check struct {
	Name    string            `yaml:"name,omitempty"`
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
	Timeout Duration          `yaml:"timeout,omitempty"`
	Retries *int              `yaml:"retries,omitempty"`
	// TLSMinDays turns the check WARN when the certificate expires sooner.
	TLSMinDays *int `yaml:"tls_min_days,omitempty"`
	// Insecure skips certificate verification (self-signed services).
	Insecure bool `yaml:"insecure,omitempty"`
}

// File is the on-disk format. "sites" is the v0 format, kept for compatibility.
type File struct {
	Defaults Defaults `yaml:"defaults,omitempty"`
	Checks   []Check  `yaml:"checks"`
	Sites    []Check  `yaml:"sites,omitempty"`
}

// Config is a validated set of checks, with every default resolved.
type Config struct {
	Checks []Check
}

// Load reads and validates a check file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Parse decodes YAML, rejecting unknown keys so that typos never pass silently.
func Parse(data []byte) (*Config, error) {
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return f.Resolve()
}

// FromURLs builds a configuration from bare URLs given on the command line.
func FromURLs(urls []string) (*Config, error) {
	f := File{}
	for _, u := range urls {
		f.Checks = append(f.Checks, Check{URL: u})
	}
	return f.Resolve()
}

// Resolve applies defaults and validates every check.
func (f File) Resolve() (*Config, error) {
	all := append(append([]Check{}, f.Checks...), f.Sites...)
	if len(all) == 0 {
		return nil, errors.New("no checks defined (add a \"checks:\" list)")
	}
	var errs []error
	seen := map[string]int{}
	for i := range all {
		c := &all[i]
		f.Defaults.apply(c)
		if err := c.validate(); err != nil {
			errs = append(errs, fmt.Errorf("check %d (%s): %w", i+1, c.label(), err))
			continue
		}
		seen[c.Name]++
		if seen[c.Name] == 2 {
			errs = append(errs, fmt.Errorf("check %d: duplicate name %q", i+1, c.Name))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &Config{Checks: all}, nil
}

func (d Defaults) apply(c *Check) {
	c.URL = strings.TrimSpace(c.URL)
	c.Method = strings.ToUpper(strings.TrimSpace(c.Method))
	if c.Method == "" {
		c.Method = DefaultMethod
	}
	if c.Name == "" {
		c.Name = c.URL
	}
	if c.Timeout.Duration == 0 {
		c.Timeout = d.Timeout
	}
	if c.Timeout.Duration == 0 {
		c.Timeout.Duration = DefaultTimeout
	}
	if c.Retries == nil {
		c.Retries = d.Retries
	}
	if c.Retries == nil {
		zero := 0
		c.Retries = &zero
	}
	if c.TLSMinDays == nil {
		c.TLSMinDays = d.TLSMinDays
	}
	if c.TLSMinDays == nil {
		v := DefaultTLSMinDays
		c.TLSMinDays = &v
	}
	if len(d.Headers) > 0 {
		h := make(map[string]string, len(d.Headers)+len(c.Headers))
		for k, v := range d.Headers {
			h[k] = v
		}
		for k, v := range c.Headers {
			h[k] = v
		}
		c.Headers = h
	}
}

func (c Check) label() string {
	if c.Name != "" {
		return c.Name
	}
	return "no name"
}

var methods = map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true}

func (c Check) validate() error {
	if c.URL == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("url %q must be an absolute http(s) URL", c.URL)
	}
	if !methods[c.Method] {
		return fmt.Errorf("unsupported method %q", c.Method)
	}
	if c.Timeout.Duration < 0 {
		return fmt.Errorf("timeout must be positive, got %s", c.Timeout)
	}
	if r := *c.Retries; r < 0 || r > MaxRetries {
		return fmt.Errorf("retries must be between 0 and %d, got %d", MaxRetries, r)
	}
	if *c.TLSMinDays < 0 {
		return fmt.Errorf("tls_min_days must not be negative, got %d", *c.TLSMinDays)
	}
	return nil
}
