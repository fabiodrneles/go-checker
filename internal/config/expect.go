package config

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/fabiodrneles/go-checker/internal/jsonpath"
	"gopkg.in/yaml.v3"
)

// Expect lists what a passing response looks like (spec 004). Without
// Status, any status below 400 passes.
type Expect struct {
	Status       StatusList        `yaml:"status,omitempty"`
	MaxLatency   Duration          `yaml:"max_latency,omitempty"`
	BodyContains string            `yaml:"body_contains,omitempty"`
	BodyRegex    string            `yaml:"body_regex,omitempty"`
	JSON         []JSONExpect      `yaml:"json,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty"`

	bodyRegex *regexp.Regexp
}

// JSONExpect asserts on one value of a JSON body. Without Equals, the path
// only has to exist (or, with exists: false, to be absent).
type JSONExpect struct {
	Path   string `yaml:"path"`
	Equals any    `yaml:"equals,omitempty"`
	Exists *bool  `yaml:"exists,omitempty"`

	hasEquals bool
	path      jsonpath.Path
}

// UnmarshalYAML records whether "equals" was written, so that "equals: null"
// differs from no equals at all.
func (j *JSONExpect) UnmarshalYAML(n *yaml.Node) error {
	type plain JSONExpect
	if err := n.Decode((*plain)(j)); err != nil {
		return err
	}
	// Decode inside a custom unmarshaler does not inherit KnownFields:
	// reject unknown keys here, as everywhere else.
	for i := 0; i+1 < len(n.Content); i += 2 {
		switch k := n.Content[i]; k.Value {
		case "equals":
			j.hasEquals = true
		case "path", "exists":
		default:
			return fmt.Errorf("line %d: field %s not found in expect.json (use path, equals, exists)", k.Line, k.Value)
		}
	}
	return nil
}

// HasEquals reports whether the check compares the value.
func (j JSONExpect) HasEquals() bool { return j.hasEquals }

// Compiled returns the parsed path.
func (j JSONExpect) Compiled() jsonpath.Path { return j.path }

// BodyRegexp returns the compiled body_regex, or nil.
func (e Expect) BodyRegexp() *regexp.Regexp { return e.bodyRegex }

// StatusList accepts "status: 200" and "status: [200, 204]".
type StatusList []int

// UnmarshalYAML decodes one code or a list.
func (s *StatusList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.SequenceNode {
		var l []int
		if err := n.Decode(&l); err != nil {
			return fmt.Errorf("line %d: status must be a number or a list of numbers", n.Line)
		}
		*s = l
		return nil
	}
	var v int
	if err := n.Decode(&v); err != nil {
		return fmt.Errorf("line %d: status must be a number or a list of numbers", n.Line)
	}
	*s = StatusList{v}
	return nil
}

// Has reports whether code is in the list.
func (s StatusList) Has(code int) bool {
	for _, c := range s {
		if c == code {
			return true
		}
	}
	return false
}

func (e *Expect) compile() error {
	var errs []error
	for _, c := range e.Status {
		if c < 100 || c > 599 {
			errs = append(errs, fmt.Errorf("expect.status %d is not an HTTP status", c))
		}
	}
	if e.MaxLatency.Duration < 0 {
		errs = append(errs, errors.New("expect.max_latency must be positive"))
	}
	if e.BodyRegex != "" {
		re, err := regexp.Compile(e.BodyRegex)
		if err != nil {
			errs = append(errs, fmt.Errorf("expect.body_regex: %w", err))
		}
		e.bodyRegex = re
	}
	for i := range e.JSON {
		j := &e.JSON[i]
		p, err := jsonpath.Parse(j.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("expect.json[%d]: %w", i, err))
		}
		j.path = p
		if j.hasEquals && j.Exists != nil && !*j.Exists {
			errs = append(errs, fmt.Errorf("expect.json[%d]: equals and exists: false contradict each other", i))
		}
	}
	return errors.Join(errs...)
}
