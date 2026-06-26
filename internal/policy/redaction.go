package policy

import (
	"regexp"
	"strings"
)

// redactKeywords are substrings whose presence marks content sensitive.
var redactKeywords = []string{
	"token", "secret", "password", "api_key", "apikey",
	"authorization", "bearer", "cookie", "private_key", "app_secret",
}

// kvPattern matches "key: value" / "key=value" / "key": "value" forms where the
// key contains a sensitive keyword. The value is captured for masking.
var kvPattern = regexp.MustCompile(
	`(?i)([A-Za-z0-9_\-]*(?:token|secret|password|api[_-]?key|authorization|bearer|cookie|private_key|app_secret)[A-Za-z0-9_\-]*)(\s*["']?\s*[:=]\s*["']?)([^\s"',}]+)`,
)

// bearerPattern matches inline bearer tokens.
var bearerPattern = regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9._\-]+)`)

const mask = "***REDACTED***"

// IsSensitive reports whether text likely contains a secret.
func IsSensitive(text string) bool {
	low := strings.ToLower(text)
	for _, k := range redactKeywords {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// IsSensitivePath reports whether a path should never be captured to disk.
func IsSensitivePath(path string) bool {
	base := path
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		base = path[i+1:]
	}
	low := strings.ToLower(base)
	switch {
	case low == ".env" || strings.HasPrefix(low, ".env."):
		return true
	case strings.Contains(low, "secret"):
		return true
	case strings.Contains(low, "credential"):
		return true
	case strings.HasSuffix(low, ".pem") || strings.HasSuffix(low, ".key"):
		return true
	}
	return false
}

// Redact masks secret-looking values in text. Deterministic.
// Bearer tokens are masked first so "Authorization: Bearer <tok>" loses the
// token rather than the literal word "Bearer".
func Redact(text string) string {
	out := bearerPattern.ReplaceAllString(text, "${1}"+mask)
	out = kvPattern.ReplaceAllString(out, "$1$2"+mask)
	return out
}
