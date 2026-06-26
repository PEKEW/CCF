package feishu

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMockClientLifecycle(t *testing.T) {
	root := t.TempDir()
	c := NewMockClient(root)
	ctx := context.Background()

	fr, err := c.CreateSessionFolder(ctx, "CC Session - Untitled", "")
	if err != nil {
		t.Fatal(err)
	}
	if fr.Token == "" {
		t.Fatal("empty folder token")
	}

	dr, err := c.CreateDoc(ctx, fr.Token, "00_SESSION_INDEX", "# Index\n")
	if err != nil {
		t.Fatal(err)
	}

	if err := c.AppendDoc(ctx, dr.Token, "appended line"); err != nil {
		t.Fatal(err)
	}
	if err := c.RenameFolder(ctx, fr.Token, "S-20260626-1504__add-auth"); err != nil {
		t.Fatal(err)
	}

	// Verify on disk via a FRESH client (simulates separate hook process).
	c2 := NewMockClient(root)
	if err := c2.UpdateDoc(ctx, dr.Token, "# Replaced\n"); err != nil {
		t.Fatalf("cross-process update failed: %v", err)
	}

	// rename file marker exists
	var found bool
	_ = filepath.Walk(root, func(p string, info os.FileInfo, _ error) error {
		if info != nil && info.Name() == "_FOLDER_NAME.txt" {
			b, _ := os.ReadFile(p)
			if strings.Contains(string(b), "add-auth") {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Fatal("rename marker not written")
	}
}

func TestMockUnknownToken(t *testing.T) {
	c := NewMockClient(t.TempDir())
	if err := c.UpdateDoc(context.Background(), "doc_missing", "x"); err == nil {
		t.Fatal("expected error for unknown doc")
	}
}
