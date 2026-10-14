// Package assert checks a response against the expectations of a check.
package assert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/probe"
)

// Severity of a failed assertion.
type Severity int

// A slow answer is a warning; a wrong one is down (spec 004 FR-4).
const (
	Warn Severity = iota + 1
	Down
)

// Failure is one assertion that did not hold.
type Failure struct {
	Severity Severity
	Reason   string
}

// Evaluate runs every assertion and returns those that failed.
func Evaluate(e config.Expect, r *probe.Response) []Failure {
	var fs []Failure
	down := func(format string, a ...any) { fs = append(fs, Failure{Down, fmt.Sprintf(format, a...)}) }

	switch {
	case len(e.Status) > 0 && !e.Status.Has(r.StatusCode):
		down("status %d, want %s", r.StatusCode, orList(e.Status))
	case len(e.Status) == 0 && r.StatusCode >= 400:
		down("status %d", r.StatusCode)
	}
	if e.MaxLatency.Duration > 0 && r.Latency > e.MaxLatency.Duration {
		fs = append(fs, Failure{Warn, fmt.Sprintf("latency %s above max_latency %s", r.Latency.Round(1e6), e.MaxLatency)})
	}
	if e.BodyContains != "" && !bytes.Contains(r.Body, []byte(e.BodyContains)) {
		down("body does not contain %q", e.BodyContains)
	}
	if re := e.BodyRegexp(); re != nil && !re.Match(r.Body) {
		down("body does not match /%s/", e.BodyRegex)
	}
	for name, want := range e.Headers {
		got := r.Header.Get(name)
		if !strings.Contains(got, want) {
			down("header %s is %q, want it to contain %q", name, got, want)
		}
	}
	if len(e.JSON) > 0 {
		var doc any
		if err := json.Unmarshal(r.Body, &doc); err != nil {
			down("body is not JSON: %v", err)
			return fs
		}
		for _, j := range e.JSON {
			got, ok := j.Compiled().Get(doc)
			switch {
			case j.Exists != nil && !*j.Exists:
				if ok {
					down("%s exists, want it absent", j.Path)
				}
			case !ok:
				down("%s not found", j.Path)
			case j.HasEquals() && !Equal(got, j.Equals):
				down("%s is %s, want %s", j.Path, show(got), show(j.Equals))
			}
		}
	}
	return fs
}

// Equal compares a decoded JSON value with a YAML one: numbers by value
// (YAML 200 equals JSON 200.0), everything else structurally.
func Equal(jsonVal, yamlVal any) bool {
	if a, ok := number(jsonVal); ok {
		b, ok := number(yamlVal)
		return ok && a == b
	}
	switch y := yamlVal.(type) {
	case []any:
		arr, ok := jsonVal.([]any)
		if !ok || len(arr) != len(y) {
			return false
		}
		for i := range y {
			if !Equal(arr[i], y[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		obj, ok := jsonVal.(map[string]any)
		if !ok || len(obj) != len(y) {
			return false
		}
		for k, v := range y {
			if !Equal(obj[k], v) {
				return false
			}
		}
		return true
	}
	return jsonVal == yamlVal
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n)
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func orList(l config.StatusList) string {
	parts := make([]string, len(l))
	for i, c := range l {
		parts[i] = strconv.Itoa(c)
	}
	return strings.Join(parts, " or ")
}
