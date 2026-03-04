package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindImage_findsJpeg(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "photo.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := findImage(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join(dir, "photo.jpg") {
		t.Errorf("got %q, want %q", got, filepath.Join(dir, "photo.jpg"))
	}
}

func TestFindImage_findsPng(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "image.png"))
	f.Close()

	got, err := findImage(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Ext(got) != ".png" {
		t.Errorf("expected .png, got %q", got)
	}
}

func TestFindImage_noImageReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := findImage(dir)
	if err == nil {
		t.Error("expected error when no image found, got nil")
	}
}

func TestFindImage_ignoresNonImageFiles(t *testing.T) {
	dir := t.TempDir()
	os.Create(filepath.Join(dir, "readme.txt"))
	os.Create(filepath.Join(dir, "data.json"))

	_, err := findImage(dir)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestFindImage_expandsTilde(t *testing.T) {
	// Verify it doesn't panic on a tilde path
	_, _ = findImage("~/nonexistent-RageQuit-test-dir-xyz")
	// No panic = pass
}
