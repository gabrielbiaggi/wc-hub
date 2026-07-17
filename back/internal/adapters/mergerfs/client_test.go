package mergerfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseAndOpenStayInsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "media", "movie.txt"), []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	items, err := client.Browse(context.Background(), "media")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Path != "media/movie.txt" {
		t.Fatalf("unexpected entries: %#v", items)
	}
	file, _, err := client.Open("media/movie.txt")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, _, err = client.Open("../outside.txt"); !errors.Is(err, ErrPathDenied) {
		t.Fatalf("expected traversal denial, got %v", err)
	}
}

func TestSymlinkEscapeIsDenied(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	client, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = client.Open("escape.txt"); !errors.Is(err, ErrPathDenied) {
		t.Fatalf("expected symlink escape denial, got %v", err)
	}
}
