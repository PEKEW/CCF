package policy

import "strings"

// validationMarkers are substrings that identify a test/validation command.
var validationMarkers = []string{
	"pytest", "go test", "cargo test", "npm test", "pnpm test",
	"yarn test", "make test", "unittest", "coverage", "jest", "vitest",
}

// IsValidationCommand reports whether a Bash command runs tests/validation.
func IsValidationCommand(cmd string) bool {
	low := strings.ToLower(cmd)
	for _, m := range validationMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}
