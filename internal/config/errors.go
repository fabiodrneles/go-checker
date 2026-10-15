package config

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var unknownField = regexp.MustCompile(`^line (\d+): field (\S+) not found in type config\.(\w+)$`)

// friendly rewrites yaml.v3 errors about unknown keys into messages that name
// the key and suggest the closest valid one ("did you mean timeout?").
func friendly(err error) error {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return err
	}
	msgs := make([]string, 0, len(te.Errors))
	for _, e := range te.Errors {
		m := unknownField.FindStringSubmatch(e)
		if m == nil {
			msgs = append(msgs, e)
			continue
		}
		msg := fmt.Sprintf("line %s: unknown key %q", m[1], m[2])
		if s := suggest(m[2], keysOf(m[3])); s != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", s)
		}
		msgs = append(msgs, msg)
	}
	return errors.New(strings.Join(msgs, "; "))
}

var types = map[string]reflect.Type{
	"File": reflect.TypeOf(File{}), "Defaults": reflect.TypeOf(Defaults{}), "Check": reflect.TypeOf(Check{}),
	"Expect": reflect.TypeOf(Expect{}), "Auth": reflect.TypeOf(Auth{}),
}

func keysOf(typeName string) []string {
	t, ok := types[typeName]
	if !ok {
		return nil
	}
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		if k := strings.Split(t.Field(i).Tag.Get("yaml"), ",")[0]; k != "" && k != "-" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// suggest returns the key closest to s, if it is close enough to be a typo.
func suggest(s string, keys []string) string {
	best, bestD := "", 3
	for _, k := range keys {
		if len(s) >= 5 && strings.HasPrefix(k, s[:len(s)-1]) {
			return k // "retry" → "retries"
		}
	}
	for _, k := range keys {
		if d := distance(s, k); d < bestD {
			best, bestD = k, d
		}
	}
	return best
}

// distance is the Levenshtein edit distance.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
