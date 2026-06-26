package session

import (
	"strings"
	"testing"
	"time"
)

func TestSlugEnglish(t *testing.T) {
	got := Slug("Add user authentication to the login flow please now extra")
	// max 6 ASCII tokens
	if n := len(strings.Split(got, "-")); n > 6 {
		t.Fatalf("too many tokens: %q (%d)", got, n)
	}
	if got != "add-user-authentication-to-the-login" {
		t.Fatalf("unexpected slug: %q", got)
	}
}

func TestSlugStripsPathsAndPunct(t *testing.T) {
	got := Slug("Fix bug in /usr/local/src/main.go!!!")
	if strings.Contains(got, "/") || strings.Contains(got, "!") {
		t.Fatalf("slug retained path/punct: %q", got)
	}
	if got == "" || got == "untitled" {
		t.Fatalf("slug empty: %q", got)
	}
}

func TestSlugChinese(t *testing.T) {
	got := Slug("添加用户登录功能并修复一些其他的小问题继续扩展更多内容")
	// at most 12 CJK chars
	if n := len([]rune(got)); n > 12 {
		t.Fatalf("too many CJK chars: %q (%d)", got, n)
	}
	if !strings.HasPrefix(got, "添加用户登录") {
		t.Fatalf("unexpected cjk slug: %q", got)
	}
}

func TestSlugEmptyFallback(t *testing.T) {
	if got := Slug("!!! @@@ ###"); got != "untitled" {
		t.Fatalf("want untitled, got %q", got)
	}
}

func TestSlugDeterministic(t *testing.T) {
	in := "Refactor the sync engine"
	if Slug(in) != Slug(in) {
		t.Fatal("slug not deterministic")
	}
}

func TestProvisionalTitle(t *testing.T) {
	got := ProvisionalTitle("Implement caching layer\nsecond line ignored")
	if got != "Implement caching layer" {
		t.Fatalf("got %q", got)
	}
}

func TestFolderName(t *testing.T) {
	tm := time.Date(2026, 6, 26, 15, 4, 0, 0, time.UTC)
	got := FolderName(tm, "add-auth")
	if got != "S-20260626-1504__add-auth" {
		t.Fatalf("got %q", got)
	}
}
