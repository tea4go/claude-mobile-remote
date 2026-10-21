package hostinfo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func randomPCID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func validPCID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil && id == strings.ToLower(id)
}

func Name() string {
	name, err := os.Hostname()
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if data, e := exec.CommandContext(ctx, "/usr/sbin/scutil", "--get", "ComputerName").Output(); e == nil && strings.TrimSpace(string(data)) != "" {
			name, err = strings.TrimSpace(string(data)), nil
		}
	}
	if err != nil || strings.TrimSpace(name) == "" || len([]rune(name)) > 255 || strings.IndexFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ""
	}
	return name
}

func PCID() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return loadOrCreatePCID(filepath.Join(configDir, "claude-mobile-remote", "pc-id"))
}

func loadOrCreatePCID(path string) (string, error) {
	read := func() (string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		id := strings.TrimSpace(string(data))
		if !validPCID(id) {
			return "", errors.New("invalid PC ID file")
		}
		return id, nil
	}
	if id, err := read(); err == nil {
		return id, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	id, err := randomPCID()
	if err != nil {
		return "", err
	}
	// Publish a complete ID atomically so concurrent CLI/WhaleTerm launches
	// cannot observe an empty file between O_EXCL creation and WriteString.
	file, err := os.CreateTemp(filepath.Dir(path), ".pc-id-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(id + "\n"); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Link(file.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return read()
		}
		return "", err
	}
	return id, nil
}
