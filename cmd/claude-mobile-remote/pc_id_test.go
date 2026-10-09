package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPCIDPersistsAcrossLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "pc-id")
	first, err := loadOrCreatePCID(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreatePCID(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !validPCID(first) {
		t.Fatalf("invalid or unstable PC ID")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
