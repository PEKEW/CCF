package policy

import (
	"path"
	"strings"
)

// norm lowercases and converts backslashes to forward slashes.
func norm(p string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"))
}

// hasDir reports whether any path segment equals dir.
func hasDir(p, dir string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == dir {
			return true
		}
	}
	return false
}

// IsTestIntegrityPath reports whether editing path touches tests, golden files,
// thresholds, eval data, CI config, or lockfiles — all of which need approval.
func IsTestIntegrityPath(p string) (bool, string) {
	n := norm(p)
	if n == "" {
		return false, ""
	}
	base := path.Base(n)

	switch {
	case hasDir(n, "tests") || hasDir(n, "test"):
		return true, "test directory"
	case hasDir(n, "golden") || hasDir(n, "testdata") || hasDir(n, "__snapshots__"):
		return true, "golden/snapshot data"
	case hasDir(n, "expected"):
		return true, "expected-output data"
	case hasDir(n, "benchmark") || hasDir(n, "benchmarks"):
		return true, "benchmark data"
	case hasDir(n, "eval") || hasDir(n, "evals"):
		return true, "eval data"
	case strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"):
		return true, "python test file"
	case strings.HasSuffix(base, "_test.go"):
		return true, "go test file"
	case strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".spec.ts") || strings.HasSuffix(base, ".spec.js"):
		return true, "js/ts test file"
	case strings.Contains(base, "threshold"):
		return true, "threshold config"
	case isCIConfig(n):
		return true, "CI configuration"
	case isLockfile(base):
		return true, "dependency lockfile"
	}
	return false, ""
}

func isCIConfig(n string) bool {
	switch {
	case strings.Contains(n, ".github/workflows/"):
		return true
	case strings.HasSuffix(n, ".gitlab-ci.yml"):
		return true
	case strings.Contains(n, ".circleci/"):
		return true
	case strings.HasSuffix(n, "azure-pipelines.yml"):
		return true
	case strings.Contains(n, ".buildkite/"):
		return true
	case path.Base(n) == "jenkinsfile":
		return true
	}
	return false
}

func isLockfile(base string) bool {
	switch base {
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"go.sum", "cargo.lock", "poetry.lock", "gemfile.lock",
		"composer.lock", "pipfile.lock":
		return true
	}
	return false
}
