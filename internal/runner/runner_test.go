package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustHash(t *testing.T, root string) string {
	t.Helper()
	h, err := sourceHash(root)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSourceHashTracksSourceOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.swift"), "let a = 1")
	writeFile(t, filepath.Join(root, "sub", "b.swift"), "let b = 2")
	base := mustHash(t, root)

	if mustHash(t, root) != base {
		t.Fatal("hash is not stable")
	}

	// Xcode per-user state and build output must not invalidate the build.
	writeFile(t, filepath.Join(root, "p.xcodeproj", "xcuserdata", "me.xcuserdatad", "x.plist"), "x")
	writeFile(t, filepath.Join(root, "Build", "out.o"), "o")
	writeFile(t, filepath.Join(root, ".DS_Store"), "d")
	if mustHash(t, root) != base {
		t.Error("ignored files changed the hash")
	}

	writeFile(t, filepath.Join(root, "sub", "b.swift"), "let b = 3")
	if mustHash(t, root) == base {
		t.Error("a content change kept the hash")
	}
}

func TestSourceHashIncludesPaths(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	writeFile(t, filepath.Join(a, "x.swift"), "same")
	writeFile(t, filepath.Join(b, "y.swift"), "same")
	if mustHash(t, a) == mustHash(t, b) {
		t.Error("a rename kept the hash")
	}
}

func TestCheckLog(t *testing.T) {
	if err := checkLog("Test Suite 'All tests' started", "log"); err != nil {
		t.Errorf("progress line reported as error: %v", err)
	}
	if err := checkLog("error: Developer App Certificate is not trusted", "log"); err == nil {
		t.Error("untrusted certificate not reported")
	}
	if err := checkLog("Testing failed:\n\tsomething", "log"); err == nil {
		t.Error("test failure not reported")
	}
}

func TestDeviceLocked(t *testing.T) {
	log := "Xcode cannot launch maestro-driver-iosUITests on Ethan's iPhone because the device is locked."
	if !deviceLocked(log) {
		t.Error("locked device not detected")
	}
	if deviceLocked("Test Suite 'All tests' started") {
		t.Error("false positive")
	}
}
