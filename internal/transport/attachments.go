package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

var attachmentCacheMu sync.Mutex

// stageAttachments creates only server-chosen paths; remote names never select
// a directory. Returned cleanup is used only before acceptance, never on Remote
// shutdown, since the host may still be reading an accepted attachment.
func stageAttachments(root string, text string, items []protocol.Attachment) (string, func(), error) {
	data, err := protocol.DecodeAttachments(items)
	if err != nil || len(strings.TrimSpace(text)) > protocol.MaxPromptBytes {
		return "", nil, protocol.ErrInvalidInput
	}
	attachmentCacheMu.Lock()
	defer attachmentCacheMu.Unlock()
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", nil, protocol.ErrInternal
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return "", nil, protocol.ErrInternal
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", nil, protocol.ErrInternal
	}
	var size int64
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return "", nil, protocol.ErrInternal
		}
		if time.Since(info.ModTime()) > 7*24*time.Hour {
			if err := os.RemoveAll(path); err != nil {
				return "", nil, protocol.ErrInternal
			}
			continue
		}
		if err := filepath.WalkDir(path, func(_ string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !d.IsDir() {
				info, e := d.Info()
				if e != nil {
					return e
				}
				size += info.Size()
			}
			return nil
		}); err != nil {
			return "", nil, protocol.ErrInternal
		}
	}
	for _, content := range data {
		size += int64(len(content))
	}
	if size > 128*1024*1024 {
		return "", nil, protocol.ErrQueueFull
	}
	dir, err := os.MkdirTemp(root, "upload-")
	if err != nil {
		return "", nil, protocol.ErrInternal
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	var prompt strings.Builder
	prompt.WriteString(strings.TrimSpace(text))
	prompt.WriteString("\n\nUser-provided attachments uploaded to this PC. Read these files using the existing tools and permissions:\n")
	for i, item := range items {
		// Keep a simple extension for reader detection, but never trust a remote path.
		ext := strings.ToLower(filepath.Ext(item.Name))
		if len(ext) > 12 || strings.IndexFunc(ext, func(r rune) bool { return r != '.' && (r < 'a' || r > 'z') && (r < '0' || r > '9') }) >= 0 {
			ext = ""
		}
		path := filepath.Join(dir, fmt.Sprintf("attachment-%d%s", i+1, ext))
		if err := os.WriteFile(path, data[i], 0600); err != nil {
			cleanup()
			return "", nil, protocol.ErrInternal
		}
		fmt.Fprintf(&prompt, "- name=%s path=%s\n", strconv.Quote(item.Name), strconv.Quote(path))
	}
	if prompt.Len() > protocol.MaxPromptBytes {
		cleanup()
		return "", nil, protocol.ErrInvalidInput
	}
	return prompt.String(), cleanup, nil
}
