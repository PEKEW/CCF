package policy

import (
	"strings"
	"testing"
)

func TestRedactKeyValue(t *testing.T) {
	cases := []string{
		`api_key=sk-12345abcde`,
		`password: hunter2`,
		`{"app_secret": "supersecretvalue"}`,
		`Authorization: Bearer abc.def.ghi`,
	}
	for _, c := range cases {
		out := Redact(c)
		if strings.Contains(out, "hunter2") || strings.Contains(out, "supersecretvalue") ||
			strings.Contains(out, "sk-12345abcde") || strings.Contains(out, "abc.def.ghi") {
			t.Errorf("secret leaked: %q -> %q", c, out)
		}
		if !strings.Contains(out, "REDACTED") {
			t.Errorf("no redaction marker: %q -> %q", c, out)
		}
	}
}

func TestRedactKeepsNonSecret(t *testing.T) {
	in := "just a normal log line about widgets"
	if Redact(in) != in {
		t.Fatalf("mangled non-secret: %q", Redact(in))
	}
}

func TestIsSensitive(t *testing.T) {
	if !IsSensitive("here is a token value") {
		t.Error("expected sensitive")
	}
	if IsSensitive("plain harmless text") {
		t.Error("unexpected sensitive")
	}
}

func TestIsSensitivePath(t *testing.T) {
	yes := []string{".env", ".env.local", "config/secrets.yaml", "id_rsa.pem", "server.key", "my-credentials.json"}
	no := []string{"main.go", "README.md", "internal/app.go"}
	for _, p := range yes {
		if !IsSensitivePath(p) {
			t.Errorf("expected sensitive path: %q", p)
		}
	}
	for _, p := range no {
		if IsSensitivePath(p) {
			t.Errorf("unexpected sensitive path: %q", p)
		}
	}
}
