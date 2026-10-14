// Package jsonpath evaluates the small JSONPath subset go-checker needs:
// $.key, $.a.b, $.items[0], $["key with spaces"] and their combinations.
package jsonpath

import (
	"fmt"
	"strconv"
	"strings"
)

// Path is a parsed expression.
type Path struct {
	src   string
	steps []step
}

type step struct {
	key   string
	index int
	isIdx bool
}

// String returns the expression as written.
func (p Path) String() string { return p.src }

// Parse compiles an expression; it must start with "$".
func Parse(s string) (Path, error) {
	p := Path{src: s}
	if !strings.HasPrefix(s, "$") {
		return p, fmt.Errorf("json path %q must start with $", s)
	}
	rest := s[1:]
	for rest != "" {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			n := strings.IndexAny(rest, ".[")
			if n < 0 {
				n = len(rest)
			}
			if n == 0 {
				return p, fmt.Errorf("json path %q: empty key", s)
			}
			p.steps = append(p.steps, step{key: rest[:n]})
			rest = rest[n:]
		case '[':
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return p, fmt.Errorf("json path %q: missing ]", s)
			}
			inner := rest[1:end]
			rest = rest[end+1:]
			if len(inner) >= 2 && (inner[0] == '"' || inner[0] == '\'') && inner[len(inner)-1] == inner[0] {
				p.steps = append(p.steps, step{key: inner[1 : len(inner)-1]})
				continue
			}
			i, err := strconv.Atoi(inner)
			if err != nil || i < 0 {
				return p, fmt.Errorf("json path %q: index %q must be a non-negative integer or a quoted key", s, inner)
			}
			p.steps = append(p.steps, step{index: i, isIdx: true})
		default:
			return p, fmt.Errorf("json path %q: unexpected %q", s, rest[0])
		}
	}
	return p, nil
}

// Get walks a decoded JSON document (encoding/json's any) and reports whether
// the path exists.
func (p Path) Get(doc any) (any, bool) {
	cur := doc
	for _, st := range p.steps {
		if st.isIdx {
			arr, ok := cur.([]any)
			if !ok || st.index >= len(arr) {
				return nil, false
			}
			cur = arr[st.index]
			continue
		}
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = obj[st.key]; !ok {
			return nil, false
		}
	}
	return cur, true
}
