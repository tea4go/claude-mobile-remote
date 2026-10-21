package hostinfo

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentHostsShareCompleteIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "pc-id")
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := loadOrCreatePCID(path)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	expected := ""
	for id := range ids {
		if !validPCID(id) {
			t.Fatal("invalid ID")
		}
		if expected != "" && id != expected {
			t.Fatal("hosts received different IDs")
		}
		expected = id
	}
	if expected == "" {
		t.Fatal("no identity created")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("identity is not private")
	}
	again, err := loadOrCreatePCID(path)
	if err != nil || again != expected {
		t.Fatal("identity did not persist")
	}
}

func TestMalformedIdentityIsNotSilentlyReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pc-id")
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreatePCID(path); err == nil {
		t.Fatal("accepted malformed identity")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "invalid" {
		t.Fatal("overwrote existing identity")
	}
}
