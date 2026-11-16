package transport

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
func stageAttachments(root string, text string, items []protocol.Attachment) (string, []string, func(), error) {
	data, err := protocol.DecodeAttachments(items)
	if err != nil || len(strings.TrimSpace(text)) > protocol.MaxPromptBytes {
		return "", nil, nil, protocol.ErrInvalidInput
	}
	attachmentCacheMu.Lock()
	defer attachmentCacheMu.Unlock()
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", nil, nil, protocol.ErrInternal
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return "", nil, nil, protocol.ErrInternal
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", nil, nil, protocol.ErrInternal
	}
	var size int64
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return "", nil, nil, protocol.ErrInternal
		}
		if time.Since(info.ModTime()) > 7*24*time.Hour {
			if err := os.RemoveAll(path); err != nil {
				return "", nil, nil, protocol.ErrInternal
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
			return "", nil, nil, protocol.ErrInternal
		}
	}
	for _, content := range data {
		size += int64(len(content))
	}
	if size > 128*1024*1024 {
		return "", nil, nil, protocol.ErrQueueFull
	}
	dir, err := os.MkdirTemp(root, "upload-")
	if err != nil {
		return "", nil, nil, protocol.ErrInternal
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	var prompt strings.Builder
	paths := make([]string, 0, len(items))
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
			return "", nil, nil, protocol.ErrInternal
		}
		paths = append(paths, path)
		fmt.Fprintf(&prompt, "- name=%s path=%s\n", strconv.Quote(item.Name), strconv.Quote(path))
	}
	if prompt.Len() > protocol.MaxPromptBytes {
		cleanup()
		return "", nil, nil, protocol.ErrInvalidInput
	}
	return prompt.String(), paths, cleanup, nil
}

// imageManifest binds the server-generated upload directory to exactly one
// session. It never contains a client-provided filesystem path.
type imageManifest struct {
	SessionHash string              `json:"sessionHash"`
	Images      []imageManifestItem `json:"images"`
}
type imageManifestItem struct {
	File      string `json:"file"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
}

func sessionHash(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])
}

// registerStagedImages writes only metadata into the existing seven-day upload
// directory. An empty reference means that item is not an image.
func registerStagedImages(sessionID string, paths []string, items []protocol.Attachment) ([]string, error) {
	if len(paths) == 0 || len(paths) != len(items) {
		return nil, protocol.ErrInternal
	}
	dir := filepath.Dir(paths[0])
	manifest := imageManifest{SessionHash: sessionHash(sessionID), Images: make([]imageManifestItem, len(items))}
	refs := make([]string, len(items))
	for i, item := range items {
		if filepath.Dir(paths[i]) != dir {
			return nil, protocol.ErrInternal
		}
		switch item.MediaType {
		case "image/jpeg", "image/png", "image/gif", "image/webp":
			manifest.Images[i] = imageManifestItem{File: filepath.Base(paths[i]), Name: item.Name, MediaType: item.MediaType}
			refs[i] = filepath.Base(dir) + ":" + strconv.Itoa(i)
		}
	}
	data, err := json.Marshal(manifest)
	if err != nil || os.WriteFile(filepath.Join(dir, "images.json"), data, 0600) != nil {
		return nil, protocol.ErrInternal
	}
	return refs, nil
}

// readStagedImage accepts an opaque upload reference, never a filesystem path.
// Expiry is enforced even if no subsequent upload has triggered cache cleanup.
func readStagedImage(root, sessionID, ref string) (protocol.Attachment, error) {
	attachmentCacheMu.Lock()
	defer attachmentCacheMu.Unlock()
	parts := strings.Split(ref, ":")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "upload-") || filepath.Base(parts[0]) != parts[0] || strings.ContainsAny(parts[0], "/\\") {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 || index >= protocol.MaxAttachments || strconv.Itoa(index) != parts[1] {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	dir := filepath.Join(root, parts[0])
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || time.Since(info.ModTime()) > 7*24*time.Hour {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	metadata, err := os.ReadFile(filepath.Join(dir, "images.json"))
	if err != nil || len(metadata) > 4096 {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	var manifest imageManifest
	if json.Unmarshal(metadata, &manifest) != nil || manifest.SessionHash != sessionHash(sessionID) || index >= len(manifest.Images) {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	item := manifest.Images[index]
	if item.File == "" || filepath.Base(item.File) != item.File || !strings.HasPrefix(item.File, "attachment-") {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	path := filepath.Join(dir, item.File)
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > protocol.MaxAttachmentBytes {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > protocol.MaxAttachmentBytes {
		return protocol.Attachment{}, protocol.ErrInvalidInput
	}
	return protocol.Attachment{Name: item.Name, MediaType: item.MediaType, Data: base64.StdEncoding.EncodeToString(data)}, nil
}
