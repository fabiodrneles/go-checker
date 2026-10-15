package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// LookupEnv reads environment variables; tests replace it.
var LookupEnv = os.LookupEnv

var envRef = regexp.MustCompile(`\$\$\{|\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

// ExpandEnv replaces ${VAR} and ${VAR:-default} so that secrets stay out of
// the file. "$${" writes a literal "${". Comment lines are left alone. A
// variable that is unset and has no default is an error naming the line.
// Only the braced form is expanded: "$" in a regex or "$.a" in a JSON path
// is never touched.
func ExpandEnv(data []byte) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	var missing []string
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		lines[i] = envRef.ReplaceAllStringFunc(line, func(m string) string {
			if m == "$${" {
				return "${"
			}
			sub := envRef.FindStringSubmatch(m)
			if v, ok := LookupEnv(sub[1]); ok && v != "" {
				return v
			}
			if sub[2] != "" {
				return sub[3]
			}
			missing = append(missing, fmt.Sprintf("line %d: environment variable %s is not set (use ${%s:-default} for a fallback)", i+1, sub[1], sub[1]))
			return m
		})
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(missing, "; "))
	}
	return []byte(strings.Join(lines, "\n")), nil
}
