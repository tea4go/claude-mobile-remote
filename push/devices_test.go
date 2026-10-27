package push

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeviceRegistryPersistsScopedTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	registry, err := LoadDeviceRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	first, second := Topic("first", "pairing-one"), Topic("second", "pairing-two")
	expiry := time.Now().Add(time.Hour)
	for _, item := range []struct{ topic, rid string }{{first, "010b81b3582"}, {first, "0207870f1b8"}, {second, "010b81b3582"}} {
		if err := registry.Register(item.topic, item.rid, expiry); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := LoadDeviceRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if ids := reloaded.Targets(first); len(ids) != 2 || ids[0] != "010b81b3582" || ids[1] != "0207870f1b8" {
		t.Fatalf("wrong first target set: %v", ids)
	}
	if ids := reloaded.Targets(second); len(ids) != 1 || ids[0] != "010b81b3582" {
		t.Fatalf("wrong second target set: %v", ids)
	}
	if err := reloaded.Remove(first, "010b81b3582"); err != nil {
		t.Fatal(err)
	}
	if ids := reloaded.Targets(first); len(ids) != 1 || ids[0] != "0207870f1b8" {
		t.Fatal("device removal changed wrong target set")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("device store is not private")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "pairing-one") || strings.Contains(string(data), "first\"") {
		t.Fatal("pairing metadata persisted")
	}
	var decoded map[string]map[string]time.Time
	if json.Unmarshal(data, &decoded) != nil {
		t.Fatal("invalid device store")
	}
}

func TestDeviceRegistryRejectsInvalidAndExpiredRoutes(t *testing.T) {
	registry, err := LoadDeviceRegistry(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	topic := Topic("session", "pairing")
	for _, item := range []struct {
		topic, rid string
		expiry     time.Time
	}{
		{"wrong", "010b81b3582", time.Now().Add(time.Hour)},
		{topic, "bad id", time.Now().Add(time.Hour)},
		{topic, "010b81b3582", time.Now().Add(-time.Second)},
		{topic, "010b81b3582", time.Now().Add(31 * 24 * time.Hour)},
	} {
		if registry.Register(item.topic, item.rid, item.expiry) == nil {
			t.Fatal("invalid registration accepted")
		}
	}
	if len(registry.Targets(topic)) != 0 {
		t.Fatal("invalid registration persisted")
	}
}
